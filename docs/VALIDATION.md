# Validation

Validated on Windows with Go 1.27.2 on 18 October 2026.

- `go test ./...`: passed for CLI, evidence, identity, scoring, and storage.
- `go build -o bin/pmiyc.exe ./cmd/pmiyc`: passed.
- Executable import of the Kevin fixture and report generation for Kevin and Laura: passed.
- Experimental fixture results under `experimental_v1`: Kevin 93.25, Laura 40.32, each with declared synthetic-source coverage 1.0.

The scores demonstrate distinct outputs under an arbitrary development rubric. They are not validation of predictive accuracy, completeness of live collection, or actual susceptibility.

Tests cover duplicate resistance, normalization idempotence, insufficient evidence, failed sources, low identity confidence, identity contradictions, missing provenance, invalid weights, reference cycles, cross-candidate support, alias collisions, preservation of existing files, and the import/review/analyze flow.

No live OSINT, model inference, or human-response study was performed. Go was downloaded to temporary storage for validation; it was not added to the system PATH. The built Windows executable is available locally under `bin/` and is ignored by Git.
