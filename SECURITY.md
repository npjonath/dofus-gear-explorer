# Security policy

## Supported versions

Only the latest release receives fixes. Installed desktop apps update themselves to it (on macOS, the app tells you a new version is available).

## Reporting a vulnerability

Please do not open a public issue. Report it privately through GitHub: **Security** tab of the repository, then **Report a vulnerability**. Include the version, your system and the steps to reproduce.

You will get an answer as soon as a maintainer can look at it. Once a fix is released, the advisory is published with credit to you if you wish.

## Scope

The desktop app runs a local API bound to `127.0.0.1` and opens no port to the network. It downloads game data from the [dofusdude/dofus3-main](https://github.com/dofusdude/dofus3-main) releases and its own updates from this repository's releases, both over HTTPS.
