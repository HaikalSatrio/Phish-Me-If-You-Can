# Phish Me If You Can

Mesin Go untuk analisis exposure dari evidence publik dalam penelitian defensif. Prototipe ini mengikuti Build Guidelines dan menyediakan evidence ingestion, review identitas, penyimpanan lokal, serta lima dimensi Personal Exposure Score (PES).

PES adalah indeks eksperimental exposure, bukan probabilitas seseorang akan terkena phishing. Bobot, threshold, dan rubric contoh belum divalidasi secara empiris.

## Menjalankan

Memerlukan Go 1.26 atau lebih baru untuk build. Executable yang sudah dibuat dapat langsung dijalankan. Jalankan dari root repository:

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

Pencarian Google tanpa API, importer URL publik, serta collector profil GitHub dan Bluesky tersedia. Pencarian sosial menggunakan filter domain pada indeks pencarian. Confidence pencocokan dan discoverability memakai heuristic eksperimental yang dapat dikonfigurasi. Kandidat wajib direview sebelum scoring. GUI, persona builder, LLM, SSR, Shapley, attack graph, dan remediation belum diimplementasikan.

## Pencarian web tanpa API

Untuk username, tersedia collector HTML langsung tanpa API serta perbandingan snapshot:

```powershell
.\bin\pmiyc.exe adapters
.\bin\pmiyc.exe search -source direct -username "HaikalSatrio" -platform github
.\bin\pmiyc.exe compare -before data/before.json -after data/after.json -out data/changes.json
```

Adapter langsung saat ini mencakup GitHub dan DEV Community. Setiap hasil menyimpan alasan deteksi, versi aturan, dan hash respons. Perbandingan tidak menganggap halaman yang gagal diakses sebagai penurunan exposure. Implementasi masih eksperimental; lihat [panduan collector langsung](docs/DIRECT_COLLECTION.md) untuk batas dukungan dan langkah penggunaan.

```powershell
.\bin\pmiyc.exe search -source web -name "Anton" -out data/anton-search.json
.\bin\pmiyc.exe candidates -input data/anton-search.json
.\bin\pmiyc.exe search -source web -name "Anton" -platform instagram
.\bin\pmiyc.exe platforms
.\bin\pmiyc.exe search -source url -url "https://example.org"
```

Sumber `web` memakai feed pencarian publik Bing tanpa API key atau browser. Hasil disaring berdasarkan kata pencarian dan domain platform. Respons HTTP berhasil yang hanya berisi hasil tidak relevan tetap dilaporkan FAILED. Pemeriksaan awal mendapatkan hasil NASA yang relevan, tetapi hasil nama pribadi tidak relevan, sehingga pencarian nama dan sosial belum bisa dijamin berhasil di lingkungan ini.

Sumber `google` tetap tersedia dan menggunakan Chrome atau Edge lokal dengan profil sementara. Tantangan Google masih dapat menghasilkan BLOCKED. Collector URL mengikuti redirect publik dengan batas request. Koleksi yang gagal tanpa kandidat menyimpan diagnostik dan mengembalikan exit code nonzero. Tanpa `-out`, nama file baru dibuat otomatis.

`-platform all` mencoba 12 platform dalam registry, dengan batas waktu dan budget. Ini adalah penemuan tautan terindeks, bukan akses langsung ke semua akun atau semua media sosial. Metadata halaman dan cuplikan pencarian tidak digunakan untuk menghitung PES. Profil yang hanya berisi metadata menghasilkan INSUFFICIENT_EVIDENCE walaupun kandidat dikonfirmasi.

Untuk GitHub gunakan `search -source github -username octocat`. Untuk Bluesky gunakan `search -source bluesky -username HANDLE`. Lihat [panduan web dan sosial](docs/WEB_AND_SOCIAL.md) serta [panduan GitHub](docs/COLLECTORS.md).

## Dokumen

- [Arsitektur](docs/ARCHITECTURE.md)
- [Spesifikasi PES](docs/PES_SPEC.md)
- [Batas riset dan roadmap](docs/RESEARCH_SCOPE.md)

Data proyek dan laporan dapat memuat informasi pribadi. Direktori `data/` tidak disertakan dalam Git. Penyimpanan versi awal tidak terenkripsi.
