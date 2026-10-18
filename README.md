# Phish Me If You Can

Mesin Go untuk analisis exposure dari evidence publik dalam penelitian defensif. Prototipe ini mengikuti Build Guidelines dan menyediakan evidence ingestion, review identitas, penyimpanan lokal, serta lima dimensi Personal Exposure Score (PES).

PES adalah indeks eksperimental exposure, bukan probabilitas seseorang akan terkena phishing. Bobot, threshold, dan rubric contoh belum divalidasi secara empiris.

## Menjalankan

Memerlukan Go 1.22 atau lebih baru. Jalankan dari root repository:

```sh
go build -o bin/pmiyc ./cmd/pmiyc
go run ./cmd/pmiyc import -input fixtures/kevin.json -out data/kevin.json
go run ./cmd/pmiyc analyze -input data/kevin.json -out data/kevin-report.json
go run ./cmd/pmiyc analyze -input fixtures/laura.json
go run ./cmd/pmiyc review -input data/kevin.json -candidate kevin-profile-other -decision REJECT -out data/kevin-reviewed.json
go test ./...
```

Pada Windows, executable build dapat diberi nama `bin/pmiyc.exe`. Semua output file harus memakai nama yang belum ada. Review menyimpan snapshot baru sehingga input asli tetap tersedia.

## Isi versi awal

- Dataset sintetis Kevin dan Laura, termasuk kandidat lain dengan nama sama.
- Validasi schema, provenance, timestamp, confidence, dan referensi evidence.
- Deduplikasi fakta observed per kandidat, dengan sumber dan ID asal tetap tersedia.
- Keputusan identitas ACCEPTED, REVIEW, ABSTAIN, atau REJECTED.
- Lima dimensi PES dan dukungan prasyarat skenario yang bisa diperiksa.
- Kebijakan scoring berversi dalam `configs/experimental-v1.json`.
- Penyimpanan snapshot JSON lokal dan output terstruktur untuk integrasi GUI mendatang.

Collector langsung, GUI, persona builder, LLM, SSR, Shapley, attack graph, dan remediation belum diimplementasikan. Semua input saat ini berasal dari JSON. Confidence dan discoverability disuplai oleh dataset atau operator, bukan dihitung dari pencarian otomatis.

## Dokumen

- [Arsitektur](docs/ARCHITECTURE.md)
- [Spesifikasi PES](docs/PES_SPEC.md)
- [Batas riset dan roadmap](docs/RESEARCH_SCOPE.md)

Data proyek dan laporan dapat memuat informasi pribadi. Direktori `data/` tidak disertakan dalam Git. Penyimpanan versi awal tidak terenkripsi.
