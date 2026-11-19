package httpapi

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/url"
	"os"
	"testing"
	"time"

	"dofusdb/api/internal/fight"
)

// The optional replay uses the ordinary HTTP interface of a local installation.
// No game artwork, database or dice seed is needed in the repository.
type speedrushReplay struct {
	t    *testing.T
	post func(string, any, any)
	req  fightRequest
	view fightDTO
}

func newSpeedrushReplay(t *testing.T, breed, room, dummies, hp int) *speedrushReplay {
	t.Helper()
	endpoint := os.Getenv("DDE_FIGHT_TEST_URL")
	if endpoint == "" {
		t.Skip("set DDE_FIGHT_TEST_URL to the running local API with DOFUS 3.7.14.18 data and art")
	}
	u, err := url.Parse(endpoint)
	if err != nil || u.Scheme != "http" || !IsLoopbackHost(u.Host) {
		t.Fatal("expected a local HTTP API URL")
	}
	client := http.Client{Timeout: 15 * time.Second}
	post := func(path string, request any, out any) {
		t.Helper()
		body, err := json.Marshal(request)
		if err != nil {
			t.Fatal(err)
		}
		res, err := client.Post(endpoint+path+"?lang=fr", "application/json", bytes.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		if res.StatusCode != 200 {
			t.Fatalf("%s returned %d", path, res.StatusCode)
		}
		if err = json.NewDecoder(res.Body).Decode(out); err != nil {
			t.Fatal(err)
		}
	}
	req := fightRequest{BreedID: breed, Dummies: &fight.Dummies{MapID: room, Count: dummies, HP: hp}}
	// The tester's published equipment IDs and point allocation, rather than a
	// frozen dump of Ankama's definitions.
	input := `{"level":150,"base":{"wisdom":181,"strength":100,"chance":100},"scrolls":{"vitality":100,"wisdom":100,"strength":100,"intelligence":100,"chance":100,"agility":100},"items":{"amulet":32202,"hat":32201,"ring1":32209,"ring2":11577,"weapon":33597,"shield":33596,"belt":33595,"cloak":32207,"boots":32208,"pet":33271,"dofus1":972,"dofus2":694,"dofus3":13344,"dofus4":13829,"dofus5":30455,"dofus6":30453},"mods":{"ring1":{"exos":[{"statId":1,"value":1}]},"ring2":{"exos":[{"statId":23,"value":1}]},"shield":{"exos":[{"statId":19,"value":1}]}}}`
	if err = json.Unmarshal([]byte(input), &req.Build); err != nil {
		t.Fatal(err)
	}
	var view fightDTO
	post("/api/fight", req, &view)
	req.Dummies = nil
	return &speedrushReplay{t: t, post: post, req: req, view: view}
}

func (r *speedrushReplay) act(a fightAction) {
	r.t.Helper()
	r.req.Action = &a
	r.req.State = r.view.State
	var result fightDTO
	r.post("/api/fight", r.req, &result)
	if result.Refused != "" {
		r.t.Fatalf("%+v refused: %s", a, result.Refused)
	}
	r.view = result
}

func (r *speedrushReplay) aim(id int) fight.Aim {
	r.t.Helper()
	req := r.req
	req.State = r.view.State
	req.Action = nil
	req.Spell = id
	var aim fight.Aim
	r.post("/api/fight/aim", req, &aim)
	return aim
}

func TestSpeedrushWithInstalledGameData(t *testing.T) {
	const player, opponent = 1, 2
	r := newSpeedrushReplay(t, 6, 99222029, 2, 1200)
	view := &r.view
	act := r.act
	if view.AP != 12 || view.MP != 5 || view.State.Player().HP != 2210 {
		t.Fatalf("wrong starting vitals")
	}
	act(fightAction{Type: "place", Entity: player, Cell: 113})
	act(fightAction{Type: "begin"})
	act(fightAction{Type: "cast", Spell: 12841, Cell: 125})
	act(fightAction{Type: "cast", Spell: 0, Cell: 125})
	if view.Names[0] != "Épée des Demoël" {
		t.Errorf("weapon name=%q", view.Names[0])
	}
	act(fightAction{Type: "cast", Spell: 24006, Cell: 125})
	act(fightAction{Type: "move", Cell: 115})
	act(fightAction{Type: "end"})
	act(fightAction{Type: "cast", Spell: 12856, Cell: 113})
	act(fightAction{Type: "end"})
	if view.State.Active().Name != "Chaton Enragé" || view.State.Active().HP != 528 || view.AP != 6 || view.MP != 4 || len(view.Spells) != 2 {
		t.Fatalf("wrong summon turn: %+v", view.State.Active())
	}
	act(fightAction{Type: "cast", Spell: 29960, Cell: 124})
	act(fightAction{Type: "end"})
	if view.State.Turn != 3 {
		t.Fatalf("turn=%d", view.State.Turn)
	}
	aim := r.aim(12841)
	for _, target := range aim.Targets {
		if target.Cell == 125 && len(target.Losses) == 1 {
			l := target.Losses[0]
			if l.Entity != opponent || l.Min != 278 || l.Max != 278 || l.CritMin != 373 || l.CritMax != 373 {
				t.Fatalf("third-turn preview: %+v", l)
			}
			return
		}
	}
	t.Fatal("missing target preview")
}
