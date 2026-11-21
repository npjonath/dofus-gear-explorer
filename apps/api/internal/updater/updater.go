// Package updater keeps the local SQLite database in sync with the game
// data releases: it detects new releases, reports that an update is
// available, and imports a release on demand (or automatically).
package updater

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"dofusdb/api/internal/game"
	"dofusdb/api/internal/rawdata"
	"dofusdb/api/internal/release"
	"dofusdb/api/internal/store"
)

type Options struct {
	DataDir                string
	FightAssetsFallbackDir string
	Langs                  []string
	ImageSize              int
	CheckInterval          time.Duration
	// AutoUpdate imports new releases as soon as they are detected.
	// When false the updater only reports that an update is available.
	AutoUpdate bool
	KeepCache  bool
}

const (
	StateIdle     = "idle"
	StateChecking = "checking"
	StateUpdating = "updating"
)

// Update steps, in order.
const (
	StepDownload = "download"
	StepParse    = "parse"
	StepImport   = "import"
	StepImages   = "images"
)

type Progress struct {
	Step    string `json:"step"`
	Percent int    `json:"percent"`
	Target  string `json:"target"`
}

type Status struct {
	State string `json:"state"`
	// Ready is true once a release has been imported and can be served.
	Ready             bool      `json:"ready"`
	InstalledVersion  string    `json:"installedVersion"`
	ImportedAt        string    `json:"importedAt"`
	LatestVersion     string    `json:"latestVersion"`
	LatestPublishedAt string    `json:"latestPublishedAt"`
	LastCheckedAt     string    `json:"lastCheckedAt"`
	UpdateAvailable   bool      `json:"updateAvailable"`
	AutoUpdate        bool      `json:"autoUpdate"`
	Progress          *Progress `json:"progress"`
	Error             string    `json:"error"`
}

// LocalVersionPrefix starts the version of data imported from a local
// directory rather than from a release.
const LocalVersionPrefix = "local-"

var ErrBusy = errors.New("an update is already running")

type Updater struct {
	store  *store.Store
	client *release.Client
	opts   Options
	log    *slog.Logger
	// onLoaded receives the dataset after every successful import.
	onLoaded func(*game.Dataset)

	mu                sync.Mutex
	state             string
	progress          *Progress
	lastError         string
	latestPublishedAt string
	// langImportFailed is the release whose language import failed in this
	// process; it is not tried again, so the release checks keep running.
	langImportFailed string
}

func New(st *store.Store, client *release.Client, opts Options, log *slog.Logger, onLoaded func(*game.Dataset)) *Updater {
	if opts.ImageSize != 64 {
		opts.ImageSize = 128
	}
	if opts.CheckInterval <= 0 {
		opts.CheckInterval = 6 * time.Hour
	}
	return &Updater{store: st, client: client, opts: opts, log: log, onLoaded: onLoaded, state: StateIdle}
}

// ImagesDir is where item icons are extracted.
func (u *Updater) ImagesDir() string { return filepath.Join(u.opts.DataDir, "images", "items") }

// FightAssetsDirs lists the local combat cache, followed by a read-only fallback if configured.
func (u *Updater) FightAssetsDirs() []string {
	primary := filepath.Join(u.opts.DataDir, "fight-assets")
	if fallback := u.opts.FightAssetsFallbackDir; fallback != "" && fallback != primary {
		return []string{primary, fallback}
	}
	return []string{primary}
}

// SpellImagesDir and BreedImagesDir hold the spell icons and the class
// portraits the fight simulator shows.
func (u *Updater) SpellImagesDir() string { return filepath.Join(u.opts.DataDir, "images", "spells") }
func (u *Updater) BreedImagesDir() string { return filepath.Join(u.opts.DataDir, "images", "breeds") }

// The archives of a release the fight simulator's images come from.
const (
	spellImagesArchive = "spell_images_96.tar.gz"
	breedImagesArchive = "class_head_images_64.tar.gz"
)

// extractFightImages unpacks the spell icons and class portraits found in a
// directory. Missing archives are not an error: the app shows initials.
func (u *Updater) extractFightImages(dir string) error {
	ds, err := u.store.Load(context.Background())
	if err != nil || ds == nil {
		return err
	}
	spells, breeds := map[int]bool{}, map[int]bool{}
	for _, sp := range ds.Spells {
		if sp.IconID > 0 {
			spells[sp.IconID] = true
		}
	}
	for _, b := range ds.Breeds {
		// Portraits are numbered by class then sex; the first is kept.
		breeds[b.ID*10] = true
	}
	for _, job := range []struct {
		archive, dest, prefix string
		wanted                map[int]bool
	}{
		{spellImagesArchive, u.SpellImagesDir(), "sort_", spells},
		{breedImagesArchive, u.BreedImagesDir(), "Head_", breeds},
	} {
		archive := filepath.Join(dir, job.archive)
		if _, statErr := os.Stat(archive); statErr != nil {
			continue
		}
		count, err := release.ExtractImages(archive, job.dest, job.prefix, job.wanted)
		if err != nil {
			return fmt.Errorf("extract %s: %w", job.archive, err)
		}
		u.log.Info("images extracted", "archive", job.archive, "count", count)
	}
	return nil
}

func (u *Updater) Status() Status {
	u.mu.Lock()
	st := Status{
		State:             u.state,
		Progress:          u.progress,
		Error:             u.lastError,
		AutoUpdate:        u.opts.AutoUpdate,
		LatestPublishedAt: u.latestPublishedAt,
	}
	u.mu.Unlock()

	st.InstalledVersion, _ = u.store.Meta(store.MetaDataVersion)
	st.ImportedAt, _ = u.store.Meta(store.MetaImportedAt)
	st.LatestVersion, _ = u.store.Meta(store.MetaLatestVersion)
	st.LastCheckedAt, _ = u.store.Meta(store.MetaLastCheckedAt)
	st.Ready = st.InstalledVersion != ""
	st.UpdateAvailable = st.LatestVersion != "" && st.LatestVersion != st.InstalledVersion
	return st
}

func (u *Updater) begin(state string) error {
	u.mu.Lock()
	defer u.mu.Unlock()
	if u.state != StateIdle {
		return ErrBusy
	}
	u.state = state
	u.lastError = ""
	return nil
}

func (u *Updater) end(err error) {
	u.mu.Lock()
	defer u.mu.Unlock()
	u.state = StateIdle
	u.progress = nil
	if err != nil {
		u.lastError = err.Error()
	}
}

func (u *Updater) setProgress(step string, percent int, target string) {
	u.mu.Lock()
	u.progress = &Progress{Step: step, Percent: percent, Target: target}
	u.mu.Unlock()
}

// Check asks GitHub for the latest release and records it.
func (u *Updater) Check(ctx context.Context) (Status, error) {
	if err := u.begin(StateChecking); err != nil {
		return u.Status(), err
	}
	rel, err := u.client.Latest(ctx)
	if err == nil {
		err = u.recordLatest(rel)
	}
	// A check that cannot reach GitHub is not a failed update: the installed
	// data keeps working offline, so the error goes to the caller only.
	u.end(nil)
	return u.Status(), err
}

func (u *Updater) recordLatest(rel *release.Release) error {
	u.mu.Lock()
	u.latestPublishedAt = rel.PublishedAt.UTC().Format(time.RFC3339)
	u.mu.Unlock()
	if err := u.store.SetMeta(store.MetaLatestVersion, rel.Tag); err != nil {
		return err
	}
	return u.store.SetMeta(store.MetaLastCheckedAt, time.Now().UTC().Format(time.RFC3339))
}

// StartUpdate imports a release in the background. An empty tag means the
// latest release. Progress is reported through Status.
func (u *Updater) StartUpdate(tag string) error {
	if err := u.begin(StateUpdating); err != nil {
		return err
	}
	go func() {
		err := u.update(context.Background(), tag)
		if err != nil {
			u.log.Error("data update failed", "error", err)
		}
		u.end(err)
	}()
	return nil
}

// Update imports a release and waits for it to finish.
func (u *Updater) Update(ctx context.Context, tag string) error {
	if err := u.begin(StateUpdating); err != nil {
		return err
	}
	err := u.update(ctx, tag)
	u.end(err)
	return err
}

func (u *Updater) update(ctx context.Context, tag string) error {
	var rel *release.Release
	var err error
	if tag == "" {
		rel, err = u.client.Latest(ctx)
		if err == nil {
			err = u.recordLatest(rel)
		}
	} else {
		rel, err = u.client.ByTag(ctx, tag)
	}
	if err != nil {
		return fmt.Errorf("find release: %w", err)
	}
	u.log.Info("importing game data", "version", rel.Tag)

	cacheDir := filepath.Join(u.opts.DataDir, "cache", rel.Tag)
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		return err
	}

	names := append([]string{}, rawdata.DataFiles...)
	for _, lang := range u.opts.Langs {
		names = append(names, rawdata.LangFile(lang))
	}
	var assets []release.Asset
	var totalBytes int64
	for _, name := range names {
		asset, ok := rel.Asset(name)
		if !ok {
			return fmt.Errorf("release %s has no %s", rel.Tag, name)
		}
		assets = append(assets, asset)
		totalBytes += asset.Size
	}

	var doneBytes int64
	for _, asset := range assets {
		base := doneBytes
		err := u.client.Download(ctx, asset, filepath.Join(cacheDir, asset.Name), func(written int64) {
			u.setProgress(StepDownload, percent(base+written, totalBytes), rel.Tag)
		})
		if err != nil {
			return err
		}
		doneBytes += asset.Size
	}

	if err := u.importDir(ctx, cacheDir, rel.Tag); err != nil {
		return err
	}

	if err := u.downloadIcons(ctx, rel, cacheDir); err != nil {
		return err
	}

	if !u.opts.KeepCache {
		_ = os.RemoveAll(filepath.Join(u.opts.DataDir, "cache"))
	}
	u.log.Info("game data imported", "version", rel.Tag)
	return nil
}

// ImportLocal imports release files already present in a directory, such as
// the output of a local doduda run. Icons are taken from an
// items_images_<size>.tar.gz archive in the same directory when there is one.
func (u *Updater) ImportLocal(ctx context.Context, dir, version string) error {
	if err := u.begin(StateUpdating); err != nil {
		return err
	}
	err := u.importDir(ctx, dir, version)
	if err == nil {
		for _, size := range []int{u.opts.ImageSize, 128, 64} {
			archive := filepath.Join(dir, fmt.Sprintf("items_images_%d.tar.gz", size))
			if _, statErr := os.Stat(archive); statErr == nil {
				err = u.extractIcons(archive, version, size)
				break
			}
		}
	}
	if err == nil {
		err = u.extractFightImages(dir)
	}
	u.end(err)
	return err
}

func (u *Updater) importDir(ctx context.Context, dir, version string) error {
	u.setProgress(StepParse, 0, version)
	ds, err := rawdata.Parse(dir, version, u.opts.Langs)
	if err != nil {
		return fmt.Errorf("parse release: %w", err)
	}
	u.setProgress(StepImport, 0, version)
	if err := u.store.Import(ctx, ds, time.Now().UTC().Format(time.RFC3339)); err != nil {
		return fmt.Errorf("import into SQLite: %w", err)
	}
	return u.Reload(ctx)
}

// extractIcons unpacks an icon archive; size is the pixel size it holds.
func (u *Updater) extractIcons(archive, version string, size int) error {
	u.setProgress(StepImages, 90, version)
	ds, err := u.store.Load(context.Background())
	if err != nil || ds == nil {
		return err
	}
	wanted := map[int]bool{}
	for _, it := range ds.Items {
		wanted[it.IconID] = true
	}
	count, err := release.ExtractIcons(archive, u.ImagesDir(), wanted)
	if err != nil {
		return fmt.Errorf("extract icons: %w", err)
	}
	u.log.Info("item icons extracted", "count", count)
	if err := u.store.SetMeta(store.MetaImagesSize, strconv.Itoa(size)); err != nil {
		return err
	}
	return u.store.SetMeta(store.MetaImagesVersion, version)
}

// iconsOutdated reports whether the icons on disk were imported at another
// size than the one asked for now.
func (u *Updater) iconsOutdated() bool {
	size, _ := u.store.Meta(store.MetaImagesSize)
	return size != strconv.Itoa(u.opts.ImageSize)
}

// langsMissing reports whether a language asked for has no imported texts.
// Data imported from a local directory is left alone: there is no release
// to download the translations from.
func (u *Updater) langsMissing(ctx context.Context, version string) bool {
	if strings.HasPrefix(version, LocalVersionPrefix) {
		return false
	}
	have, err := u.store.Languages(ctx)
	if err != nil {
		u.log.Warn("cannot read the imported languages", "error", err)
		return false
	}
	for _, lang := range u.opts.Langs {
		if !slices.Contains(have, lang) {
			return true
		}
	}
	return false
}

func (u *Updater) langImportFailedFor() string {
	u.mu.Lock()
	defer u.mu.Unlock()
	return u.langImportFailed
}

// StartLanguageImport imports a release again, in the background, to add
// the languages it lacks. The installed data keeps working meanwhile, so a
// failed import (no network, say) is logged, not reported as a failed
// update, and tried again at the next start.
func (u *Updater) StartLanguageImport(version string) error {
	if err := u.begin(StateUpdating); err != nil {
		return err
	}
	go func() {
		if err := u.update(context.Background(), version); err != nil {
			u.log.Warn("language import failed", "error", err)
			u.mu.Lock()
			u.langImportFailed = version
			u.mu.Unlock()
		}
		u.end(nil)
	}()
	return nil
}

// StartIconRefresh downloads the icons of the installed release again, in
// the background, without touching the game data.
func (u *Updater) StartIconRefresh() error {
	if err := u.begin(StateUpdating); err != nil {
		return err
	}
	go func() {
		// The icons on disk keep working, so a failed refresh (no network,
		// say) is logged and tried again at the next check, not reported as
		// a failed update.
		if err := u.refreshIcons(context.Background()); err != nil {
			u.log.Warn("icon refresh failed", "error", err)
		}
		u.end(nil)
	}()
	return nil
}

func (u *Updater) refreshIcons(ctx context.Context) error {
	version, _ := u.store.Meta(store.MetaDataVersion)
	rel, err := u.client.ByTag(ctx, version)
	if err != nil {
		return fmt.Errorf("find release: %w", err)
	}
	cacheDir := filepath.Join(u.opts.DataDir, "cache", rel.Tag)
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		return err
	}
	if err := u.downloadIcons(ctx, rel, cacheDir); err != nil {
		return err
	}
	if !u.opts.KeepCache {
		_ = os.RemoveAll(filepath.Join(u.opts.DataDir, "cache"))
	}
	return nil
}

// downloadIcons fetches the icon archive of a release and extracts it. A
// release without the archive is not an error: the app shows slot glyphs.
func (u *Updater) downloadIcons(ctx context.Context, rel *release.Release, cacheDir string) error {
	imagesName := fmt.Sprintf("items_images_%d.tar.gz", u.opts.ImageSize)
	imagesAsset, ok := rel.Asset(imagesName)
	if !ok {
		u.log.Warn("release has no item icons archive", "asset", imagesName)
		return nil
	}
	archive := filepath.Join(cacheDir, imagesName)
	err := u.client.Download(ctx, imagesAsset, archive, func(written int64) {
		// Downloading is most of the work of this step.
		u.setProgress(StepImages, percent(written, imagesAsset.Size)*9/10, rel.Tag)
	})
	if err != nil {
		return fmt.Errorf("download icons: %w", err)
	}
	if err := u.extractIcons(archive, rel.Tag, u.opts.ImageSize); err != nil {
		return err
	}
	// The simulator's images are small next to the item icons: a failed
	// download leaves the app with initials instead, not a failed import.
	for _, name := range []string{spellImagesArchive, breedImagesArchive} {
		asset, ok := rel.Asset(name)
		if !ok {
			u.log.Warn("release has no such archive", "asset", name)
			continue
		}
		if err := u.client.Download(ctx, asset, filepath.Join(cacheDir, name), func(int64) {}); err != nil {
			u.log.Warn("download failed", "asset", name, "error", err)
		}
	}
	return u.extractFightImages(cacheDir)
}

// Reload reads the dataset from SQLite and hands it to the server.
func (u *Updater) Reload(ctx context.Context) error {
	ds, err := u.store.Load(ctx)
	if err != nil {
		return fmt.Errorf("load dataset: %w", err)
	}
	if ds != nil && u.onLoaded != nil {
		u.onLoaded(ds)
	}
	return nil
}

// Run performs the startup import or check, then checks for new releases
// on every interval until ctx is cancelled.
func (u *Updater) Run(ctx context.Context) {
	cycle := func() {
		st := u.Status()
		if !st.Ready {
			if err := u.StartUpdate(""); err != nil && !errors.Is(err, ErrBusy) {
				u.log.Error("cannot start the initial import", "error", err)
			}
			return
		}
		// Languages asked for since the last import (a new default, say) are
		// imported from the release already installed, without upgrading it.
		if u.langsMissing(ctx, st.InstalledVersion) && u.langImportFailedFor() != st.InstalledVersion {
			if err := u.StartLanguageImport(st.InstalledVersion); err != nil && !errors.Is(err, ErrBusy) {
				u.log.Error("cannot start the language import", "error", err)
			}
			return
		}
		// Icons imported at another size are fetched again; the data stays.
		if u.iconsOutdated() {
			if err := u.StartIconRefresh(); err != nil && !errors.Is(err, ErrBusy) {
				u.log.Error("cannot start the icon refresh", "error", err)
			}
			return
		}
		st, err := u.Check(ctx)
		if err != nil {
			if !errors.Is(err, ErrBusy) {
				u.log.Warn("release check failed", "error", err)
			}
			return
		}
		if !st.UpdateAvailable {
			return
		}
		u.log.Info("a new game data release is available", "installed", st.InstalledVersion, "latest", st.LatestVersion)
		if u.opts.AutoUpdate {
			if err := u.StartUpdate(""); err != nil && !errors.Is(err, ErrBusy) {
				u.log.Error("cannot start the update", "error", err)
			}
		}
	}

	cycle()
	ticker := time.NewTicker(u.opts.CheckInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			cycle()
		}
	}
}

func percent(done, total int64) int {
	if total <= 0 {
		return 0
	}
	p := int(done * 100 / total)
	if p > 100 {
		p = 100
	}
	return p
}
