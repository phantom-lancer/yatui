Derived from github.com/llehouerou/go-mp3 v1.5.0 (Apache-2.0).

Local changes:
- Keep the upstream lazy decoder initialization and buffered frame parsing.
- Build the frame index only through the target frame when seeking. This avoids waiting for the entire track before a seek to an earlier position can complete.
- Preserve the original YATUI module import path to avoid changing the application API.

Upstream source: https://github.com/llehouerou/go-mp3/tree/v1.5.0
See LICENSE for the upstream Apache-2.0 license.
