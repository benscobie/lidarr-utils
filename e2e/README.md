# End-to-end test design

## Purpose

The end-to-end suite verifies `lidarr-utils` against a real Lidarr instance.
It covers the public CLI, Lidarr API integration, MusicBrainz integration,
release selection, persisted state, and filesystem mutations without calling
live third-party services.

The suite is separate from `go test ./...`. Unit tests remain the fast source
of coverage for decision branches and error cases; these tests protect the
contracts between the compiled CLI and real Lidarr behavior.

## Entry point

`./e2e/run.sh` is the single local and CI entry point. With no arguments it
runs all scenarios in parallel. Passing one scenario name runs only that
scenario for debugging:

```text
./e2e/run.sh
./e2e/run.sh monitor-artist
./e2e/run.sh monitor-labels
./e2e/run.sh dedupe
```

The script returns non-zero if any scenario fails.

## Isolation and parallelism

One Docker Compose project and one real Lidarr instance serve the whole suite.
Global setup completes before tests start: Lidarr receives one metadata-source
configuration and one root folder, and all fixture catalogues and audio files
are imported to a stable state.

The three scenario processes then run concurrently against that shared stack.
They use distinct artists, albums, MBIDs, state files, log files, and music
paths. Artist and label monitoring mutate disjoint album IDs. Dedupe scans the
shared catalogue but only its namespaced artist has imported files, preventing
it from changing either monitoring scenario's data. Assertions against
Lidarr's shared command queue filter by the scenario's album IDs rather than
relying on global command counts or ordering.

The runtime network is internal, preventing service containers from reaching
public metadata APIs after their images have been built or pulled. As the suite
grows, new scenarios share this stack by default. A test that must mutate
global Lidarr configuration belongs in the serial setup phase or behind an
explicit suite-wide synchronization point.

## Stack

`e2e/compose.yaml` defines these services:

- **Lidarr:** a real LinuxServer Lidarr image pinned to an immutable digest.
  Its config and music directories use suite-owned named volumes, with each
  scenario confined to distinct paths inside the shared music volume.
- **Fixture server:** a pinned lightweight HTTP image serving repository-owned
  Lidarr metadata and MusicBrainz JSON. Lidarr and `lidarr-utils` use only this
  service for external metadata.
- **Runner:** a test-only image which compiles the current `lidarr-utils`
  source and includes the command-line tools needed to configure Lidarr,
  generate tagged FLAC files, invoke the CLI, poll asynchronous commands, and
  assert API and filesystem state.

All base and service images are pinned by digest. Renovate may propose digest
updates, making Lidarr compatibility changes deliberate and reviewable.

## Lifecycle

The runner:

1. Waits for Lidarr to create its configuration and become healthy.
2. Reads the generated API key from the shared config volume.
3. Configures an accessible music root and redirects Lidarr metadata to the
   local fixture service.
4. Seeds the selected scenarios' disjoint artists, releases, and required
   audio.
5. Polls Lidarr imports and asynchronous setup commands to a stable baseline.
6. Launches all scenario scripts concurrently, each with its own config,
   state, log, and output files.
7. Waits for every scenario and aggregates their exit statuses instead of
   cancelling remaining tests after the first failure.
8. Makes assertions against Lidarr's API and the shared music volume.

Polling reports the operation, deadline, and last observed state on timeout.
The harness does not use long fixed sleeps as readiness or completion checks.

The host entry point captures each scenario's output plus Lidarr and
fixture-server logs before teardown when any scenario fails. It then removes
the project's containers, network, and disposable volumes whether the suite
passed or failed. Generated artifacts live under `e2e/artifacts/` and are
excluded from Git.

## Scenarios

### `monitor-artist`

The fixture catalogue contains an unmonitored album and a redundant single
for one artist. Both releases share a MusicBrainz recording ID.

The runner invokes:

```text
lidarr-utils monitor artist <artist-mbid>
```

It verifies that the album becomes monitored, the covered single remains
unmonitored, an album search is accepted by Lidarr, and the selected album is
recorded in the tool's state file.

### `monitor-labels`

A separate catalogue contains one release group issued by the fixture label
and one unrelated release. The configured `musicbrainz.url` points at the
fixture service, whose label response contains only the label-owned group.

The runner invokes:

```text
lidarr-utils monitor labels <label-mbid>
```

It verifies that the label release becomes monitored, the unrelated release
remains unmonitored, and Lidarr accepts the corresponding search. The Compose
network provides no route or configuration for the public MusicBrainz API.

### `dedupe`

A third catalogue contains downloaded album and single files for one artist.
The files share a MusicBrainz recording ID and are stored in the disposable
music volume.

The runner first invokes `lidarr-utils dedupe --dry-run` and verifies that
monitoring and both files are unchanged. It then invokes `lidarr-utils dedupe`
and verifies that Lidarr unmonitors the single and deletes its file while the
album file remains present and associated with the album.

Import exclusions are disabled in this initial scenario; they are a separate
Lidarr API contract and are not required to prove duplicate cleanup.

## Fixture boundaries

Fixture identifiers are stable, synthetic UUIDs. Each scenario uses distinct
artists, release groups, releases, recordings, and tracks, so one workflow
cannot accidentally satisfy another workflow's assertions.

The fixture server is intentionally data-only. Scenario orchestration and
assertions remain in the runner, while fixture files contain complete response
shapes observed from the pinned Lidarr metadata and MusicBrainz APIs.

No fixture or test may contact live Lidarr metadata, MusicBrainz, indexers, or
download clients. Search commands need only be accepted into Lidarr's command
queue; successful downloading is outside this suite.

## Continuous integration

The existing CI workflow gains one `e2e` job. It starts the shared stack and
runs `monitor-artist`, `monitor-labels`, and `dedupe` concurrently inside that
job. It runs for pull requests and pushes already covered by the workflow,
uses a job timeout, and uploads the suite artifacts when any scenario fails.

The existing Go quality and production-image build jobs remain independent.
No E2E scenario is folded into `go test ./...`, and no production Go behavior
is changed merely to support the harness.

## Acceptance criteria

- All three scenarios pass independently and when launched concurrently
  against one Lidarr instance.
- Repeated suites start from empty state and leave no project containers or
  volumes behind.
- Disconnecting internet access after the service and runner images are built
  or pulled does not change test behavior.
- A failed assertion identifies the scenario and phase and preserves useful
  CLI, Lidarr, and fixture-server output.
- The full existing Go test suite and vet checks remain green.
