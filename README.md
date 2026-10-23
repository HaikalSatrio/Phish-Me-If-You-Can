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

Pencarian web tanpa API key, ekstraksi halaman publik, serta collector profil GitHub, DEV Community, dan Bluesky tersedia. Pencarian sosial menggunakan filter domain pada indeks pencarian. Confidence pencocokan dan discoverability memakai heuristic eksperimental yang dapat dikonfigurasi. Kandidat wajib direview sebelum scoring. GUI, persona builder, LLM, SSR, Shapley, attack graph, dan remediation belum diimplementasikan.

## Koleksi gratis tanpa API key

Pencarian username baru menggunakan mode `auto`: API publik GitHub, DEV Community, dan Bluesky. Tidak memerlukan langganan atau API key. Untuk Bluesky, username pendek hanya dicoba sebagai `USERNAME.bsky.social`; masukkan handle lengkap untuk domain khusus. Hasil dari situs berbeda tetap menjadi kandidat terpisah.

```powershell
.\bin\pmiyc.exe search -username "YOUR_USERNAME"
.\bin\pmiyc.exe search -source auto -username "YOUR_USERNAME" -platform devto
.\bin\pmiyc.exe search -source devto -username "ben"
.\bin\pmiyc.exe search -source bluesky -username "bsky.app"
```

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

Sumber `web` mencoba SearXNG lokal di `http://127.0.0.1:8888`, lalu HTML publik DuckDuckGo dan Bing RSS selama budget dan batas kandidat masih tersedia, bukan hanya ketika sumber pertama kosong. SearXNG selalu mengirim kueri nama tanpa petunjuk lebih dulu, lalu variasi dengan perusahaan/lokasi; jumlah halaman, bahasa, engine, dan rentang waktu dapat diatur dengan `search_pages`, `search_language`, `searxng_engines`, dan `search_time_range`. Hasil harus cocok dengan nama atau username serta domain platform. Perusahaan dan lokasi digunakan untuk mengurutkan hasil, sehingga cuplikan yang tidak menyebutkannya tidak langsung dibuang. Halaman hasil kemudian diambil selama budget masih tersedia. Teks halaman dibatasi 8.000 karakter dan tautan publik maksimal 50 per halaman; CLI menampilkan pratinjau dan JSON menyimpan hasil lengkap dalam batas tersebut. Mesin pencarian tetap dapat memblokir akses atau memberikan hasil tidak relevan.

Jika field perusahaan terstruktur kosong, teks halaman, cuplikan, email, atau domain profil dapat menghasilkan `company` berstatus INFERRED. Pola yang dikenali mencakup label perusahaan eksplisit, frasa seperti "works at" atau "bekerja di", peran yang diikuti organisasi, nama universitas/perusahaan, serta domain email/profil. Nilai tersebut ditandai `(inferred, unverified)` di CLI dan `company_inferred` di JSON, didukung oleh observasi sumber, tetapi tidak dihitung sebagai match, kontradiksi, atau bukti PES. Inferensi hanya berjalan jika nama subjek muncul di sumber dan tidak menimpa field terstruktur yang sudah ada. Setiap inferensi menyimpan `inference_rule`, `inference_quote`, `inference_quote_type`, dan maksimal tiga alternatif beserta rule/confidence. Jika dua organisasi tidak terkait memiliki confidence berdekatan, atau domain dan teks mengarah ke organisasi tidak terkait, hasilnya ditandai `company_ambiguous` dan `company_alternatives`; nilai terbaik tetap ditampilkan sebagai tebakan, bukan fakta. Varian nama yang terkait atau singkatan seperti `Binus` dan `Bina Nusantara University` tidak dianggap konflik. Konteks pendidikan dan acara seperti "studied at", "alumni of", "student at", atau "spoke at" tidak dianggap workplace untuk pola nama organisasi tanpa label eksplisit. Pola nama organisasi tanpa label juga ditolak bila hanya berasal dari judul halaman, prosa artikel/penghargaan, atau prosa dengan kata penghubung; inferensi domain hanya dipakai untuk URL berstruktur profil, bukan halaman referensi atau artikel. Pola eksplisit, pekerjaan, dan peran mencakup Inggris, Indonesia, Spanyol, Prancis, Jerman, Portugis, Italia, dan Belanda, termasuk bentuk badan hukum internasional. Corpus regresi sintetis ada di `internal/extract/testdata/company_eval.json`; hasilnya adalah regression gate, bukan kalibrasi empiris.

Sumber `google` tetap tersedia dan menggunakan Chrome atau Edge lokal dengan profil sementara. Tantangan Google masih dapat menghasilkan BLOCKED. Collector URL mengikuti redirect publik dengan batas request. Koleksi yang gagal tanpa kandidat menyimpan diagnostik dan mengembalikan exit code nonzero. Tanpa `-out`, nama file baru dibuat otomatis.

Jalankan `powershell -File scripts/start-local-search.ps1` untuk menyalakan Docker Desktop bila diperlukan, menjalankan SearXNG, dan menunggu layanan siap. Setelah itu, `search -source web` langsung memakai layanan tersebut tanpa tambahan `-config`. Untuk alamat layanan lain, atur `searxng_url` pada konfigurasi collector. Lihat [langkah menjalankan SearXNG](docs/WEB_AND_SOCIAL.md#optional-local-searxng). Akses mesin pencarian di belakangnya tetap bergantung pada jaringan dan pembatasan layanan.

`-username ben` berarti mengambil satu akun dengan username persis `ben`, bukan semua orang bernama Ben. Untuk penemuan berdasarkan nama gunakan `search -source web -platform devto -name "Ben"`. Hasil pencarian web dapat mencakup profil dan artikel, bukan daftar lengkap orang. Nama dengan spasi harus diberi tanda kutip, misalnya `-name "Haikal Satrio Dewandaru"`.

`-platform all` pada mode `auto` mencoba tiga collector API publik; pada mode `direct` mencoba dua adapter HTML; pada mode `web` mencoba 13 entri registry pencarian. Batas waktu dan budget tetap berlaku. Daftar pencarian tidak berarti setiap platform memiliki adapter profil. Teks, tautan, metadata halaman, dan cuplikan pencarian tidak digunakan untuk menghitung PES. Kandidat yang hanya berisi observasi halaman menghasilkan INSUFFICIENT_EVIDENCE walaupun dikonfirmasi.

Untuk GitHub gunakan `search -source github -username octocat`. Untuk Bluesky gunakan `search -source bluesky -username HANDLE`. Lihat [panduan web dan sosial](docs/WEB_AND_SOCIAL.md) serta [panduan GitHub](docs/COLLECTORS.md).

## Dokumen

- [Arsitektur](docs/ARCHITECTURE.md)
- [Spesifikasi PES](docs/PES_SPEC.md)
- [Batas riset dan roadmap](docs/RESEARCH_SCOPE.md)

Data proyek dan laporan dapat memuat informasi pribadi. Direktori `data/` tidak disertakan dalam Git. Penyimpanan versi awal tidak terenkripsi.
