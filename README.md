# Phish Me If You Can

Mesin Go untuk analisis exposure dari evidence publik dalam penelitian defensif. Prototipe ini mengikuti Build Guidelines dan menyediakan evidence ingestion, review identitas, penyimpanan lokal, serta lima dimensi Personal Exposure Score (PES).

Evaluasi yang dapat dijalankan ulang tersedia melalui `pmiyc benchmark -out data/benchmark.json`. Tambahkan `-model qwen2.5:7b` untuk membandingkan rules dan Ollama pada input identik. Corpus awal berisi 24 kasus sintetis, bukan ukuran akurasi web. Lihat [panduan benchmark](docs/BENCHMARK.md).

Setelah konfirmasi identitas, `pmiyc profile -input data/hasil-reviewed.json -out data/profile-report.json` menggabungkan klaim dari profil terkonfirmasi dengan kutipan, periode, sumber, dan penanda employer yang perlu diperiksa. Lihat [ringkasan profil](docs/PROFILE-SUMMARY.md).

Hubungan nama panggung dan nama publik sekarang diekstrak dari kutipan eksplisit tanpa Ollama. Nama yang ambigu tetap dipisahkan untuk review, dan pencarian nama saja dapat menambahkan query biografi dalam budget yang sama. Lihat [cara kerja dan hasil uji 17 query](docs/PUBLIC-NAMES.md).

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

Pencarian web tanpa API key, ekstraksi halaman publik, serta collector profil GitHub, DEV Community, dan Bluesky tersedia. Pencarian sosial menggunakan filter domain pada indeks pencarian. Confidence pencocokan dan discoverability memakai heuristic eksperimental yang dapat dikonfigurasi. Kandidat wajib direview sebelum scoring. Ekstraksi hubungan dan LLM lokal opsional tersedia. GUI, persona builder, SSR, Shapley, attack graph, dan remediation belum diimplementasikan.

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
.\bin\pmiyc.exe search -source web -name "Haikal" -account "instagram:@haikal_satrio" -account "github:HaikalSatrio"
.\bin\pmiyc.exe next -input data/anton-search.json
.\bin\pmiyc.exe platforms
.\bin\pmiyc.exe search -source url -url "https://example.org"
```

Sumber `web` mencoba URL atau halaman akun yang diberikan lebih dulu, lalu SearXNG lokal di `http://127.0.0.1:8888`, HTML publik DuckDuckGo, dan Bing RSS selama budget dan batas kandidat masih tersedia. Ulangi `-account platform:username` untuk maksimal delapan petunjuk akun; setiap akun dicari pada platformnya sendiri dan tetap menjadi kandidat terpisah. Petunjuk tersimpan saat sesi dilanjutkan. Jangan gabungkan `-account` dengan `-username` atau `-platform`; gunakan pasangan platform dan username pada setiap flag. Untuk satu akun, `-username` dengan `-platform` tetap tersedia. URL eksplisit dapat dipakai melalui `-source web -url` atau `-source url -url`. Halaman hasil tebakan URL akun harus menyebut handle pada metadata dan tidak boleh berpindah ke akun lain. Kecocokan ini belum memverifikasi kepemilikan.

Pada indeks, username menjadi batas pencarian jika diberikan; kecocokan nama saja tidak cukup. Awalan `@` boleh digunakan. Tanpa petunjuk akun atau username, pencarian memakai nama. SearXNG mengirim kueri utama lebih dulu, lalu variasi dengan nama, perusahaan, atau lokasi, maksimal empat kueri per akun; jumlah halaman, bahasa, engine, dan rentang waktu dapat diatur dengan `search_pages`, `search_language`, `searxng_engines`, dan `search_time_range`. Perusahaan dan lokasi mengurutkan hasil tanpa membuang cuplikan yang tidak menyebutkannya. Semua tahap berbagi budget request dan batas kandidat. Teks halaman dibatasi 8.000 karakter dan tautan publik maksimal 50 per halaman. Mesin pencarian dapat memblokir akses atau memberikan hasil tidak relevan; hasil kosong belum membuktikan akun tidak ada.

Setelah pencarian, CLI menampilkan pertanyaan tindak lanjut sesuai bukti yang belum jelas. `pmiyc next -input data/hasil.json` menampilkan ulang pertanyaan tanpa request jaringan; tambahkan `-out data/pertanyaan.json` untuk JSON beserta kartu kandidat dan klaim sumber. Jalankan `pmiyc challenge -input data/hasil.json -out data/hasil-reviewed.json` untuk memilih profil secara interaktif, atau tambahkan `-interactive` pada pencarian baru. Pengguna dapat mengaitkan beberapa profil, menolak semuanya, atau menyatakan belum yakin. Keputusan dan alasannya disimpan tanpa mengubah snapshot asli. Identitas, employer, dan periode direview secara terpisah. Lihat [alur konfirmasi identitas](docs/IDENTITY-REVIEW.md) dan [validasi akun dan kualitas ekstraksi](docs/COMPARISON-ACCOUNT-QUALITY.md).

Ekstraksi menyimpan hubungan beserta subjek, kutipan, sumber, dan periode. Jabatan publik historis dipisahkan dari pekerjaan saat ini; pendidikan, keanggotaan, dan tempat lahir tidak menjadi company. Company/role diringkas dari hubungan deterministik atau usulan yang diterima operator. Beberapa role dan riwayat tampil sebagai klaim terpisah dengan referensi sumber; perusahaan yang berbeda tetap ambigu. Semua klaim tetap unverified dan tidak memengaruhi confidence identitas atau PES.

Company tidak dinonaktifkan dan tidak memerlukan input `-company` agar dapat terisi. Output menjelaskan field kosong, sedangkan JSON menyimpan `company_status`: misalnya `no_employment_claim`, `historical_only`, `ambiguous`, `period_conflict`, atau `pending_review`. Status ini menjelaskan hasil ekstraksi, bukan keadaan pekerjaan orang tersebut.

Gunakan `pmiyc enrich -input data/hasil-lama.json -out data/hasil-relasi.json` untuk memproses ulang snapshot. Tambahkan `-model qwen2.5:7b` untuk model lokal yang sudah dipasang, dan `-debug-extraction` jika memerlukan respons mentah. Alasan penolakan model dicatat per usulan. Perintah `review-relation` menyediakan keputusan ACCEPT/REJECT/PENDING tanpa mengubah input asli. Model tidak diunduh otomatis dan tidak memerlukan API berbayar. Lihat [panduan ekstraksi dan LLM lokal](docs/EXTRACTION.md) untuk konfigurasi, review, periode, serta batas validasi.

Mode tanpa Ollama mendukung pola pekerjaan dan headline yang umum, termasuk bentuk posesif dan nama organisasi setelah `of` atau `for`. Mode Ollama memilih potongan sumber dengan ID pendek agar aplikasi dapat memasang kutipan asli. Keduanya dapat dibandingkan pada snapshot yang sama; lihat [perbandingan tiga nama baru](docs/COMPARISON-NEW-NAMES.md) untuk hasil nyata dan keterbatasannya.

Sumber `google` tetap tersedia dan menggunakan Chrome atau Edge lokal dengan profil sementara. Tantangan Google masih dapat menghasilkan BLOCKED. Collector URL mengikuti redirect publik dengan batas request. Koleksi yang gagal tanpa kandidat menyimpan diagnostik dan mengembalikan exit code nonzero. Tanpa `-out`, nama file baru dibuat otomatis.

Jalankan `powershell -File scripts/start-local-search.ps1` untuk menyalakan Docker Desktop bila diperlukan, menjalankan SearXNG, dan menunggu layanan siap. Setelah itu, `search -source web` langsung memakai layanan tersebut tanpa tambahan `-config`. Untuk alamat layanan lain, atur `searxng_url` pada konfigurasi collector. Lihat [langkah menjalankan SearXNG](docs/WEB_AND_SOCIAL.md#optional-local-searxng). Akses mesin pencarian di belakangnya tetap bergantung pada jaringan dan pembatasan layanan.

`-username ben` berarti mengambil satu akun dengan username persis `ben`, bukan semua orang bernama Ben. Untuk penemuan berdasarkan nama gunakan `search -source web -platform devto -name "Ben"`. Hasil pencarian web dapat mencakup profil dan artikel, bukan daftar lengkap orang. Nama dengan spasi harus diberi tanda kutip, misalnya `-name "Haikal Satrio Dewandaru"`.

`-platform all` pada mode `auto` mencoba tiga collector API publik; pada mode `direct` mencoba dua adapter HTML; pada mode `web` mencoba 13 entri registry pencarian. Batas waktu dan budget tetap berlaku. Daftar pencarian tidak berarti setiap platform memiliki adapter profil. Teks, tautan, metadata halaman, dan cuplikan pencarian tidak digunakan untuk menghitung PES. Kandidat yang hanya berisi observasi halaman menghasilkan INSUFFICIENT_EVIDENCE walaupun dikonfirmasi.

Untuk GitHub gunakan `search -source github -username octocat`. Untuk Bluesky gunakan `search -source bluesky -username HANDLE`. Lihat [panduan web dan sosial](docs/WEB_AND_SOCIAL.md) serta [panduan GitHub](docs/COLLECTORS.md).

## Dokumen

- [Penelusuran bertahap, bukti identitas, dan pendidikan](docs/ITERATIVE-RESEARCH.md)
- [Arsitektur](docs/ARCHITECTURE.md)
- [Spesifikasi PES](docs/PES_SPEC.md)
- [Batas riset dan roadmap](docs/RESEARCH_SCOPE.md)

Data proyek dan laporan dapat memuat informasi pribadi. Direktori `data/` tidak disertakan dalam Git. Penyimpanan versi awal tidak terenkripsi.
