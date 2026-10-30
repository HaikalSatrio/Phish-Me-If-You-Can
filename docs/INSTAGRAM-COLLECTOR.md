# Collector Instagram Go

Integrasi 30 Oktober 2026. Mengadaptasi subset anonim Instaloader 4.16 dari prototipe Go terpisah; bukan port seluruh fitur Instaloader. Lisensi sumber ada di [instaloader-LICENSE](../third_party/instaloader-LICENSE).

**Perbaikan caption:** collector kini membaca caption yang sudah ada dalam timeline HTML profil terlebih dahulu. Uji langsung `YOUR_USERNAME` berhasil mengambil caption **“[caption akun uji disamarkan]”** dengan satu request dan status postingan `SUCCESS`. Sebelumnya caption tersebut diabaikan, lalu request detail tambahan terkena 429.

## Jalankan di WSL

Dari root repository:

```bash
go build -o bin/pmiyc ./cmd/pmiyc

# Satu username yang sudah diketahui; config lama tetap bisa digunakan.
./bin/pmiyc search -source instagram -username YOUR_USERNAME \
  -out data/instagram-search.json

# Alternatif: URL profil.
./bin/pmiyc search -source url \
  -url 'https://www.instagram.com/YOUR_USERNAME/' \
  -out data/instagram-url.json

# Gabungkan dengan pencarian web. Setiap akun tetap kandidat tersendiri.
./bin/pmiyc search -source web -name 'NAMA SUBJEK' \
  -account instagram:USERNAME \
  -out data/web-instagram.json
```

Gunakan nama output baru setiap run, atau hapus opsi `-out` agar nama dibuat otomatis. Tidak perlu mengaktifkan Python/venv, menjalankan prototipe, atau menyetel environment variable. Mode `auto` tetap memakai sumber API publik yang sebelumnya tersedia; Instagram dipanggil lewat jalur di atas. URL postingan langsung belum memakai adapter Instagram ini: input adapter adalah profil.

Default **maksimal 3 postingan**, mengikuti urutan yang diberikan Instagram, bukan jaminan tiga postingan terbaru. Tambahkan field berikut ke salinan config collector lengkap jika perlu mengubah batas; nilai yang diterima 1–12:

```json
"instagram": { "max_posts": 3 }
```

Lalu gunakan `-config configs/NAMA-CONFIG.json`. Budget request dan timeout tetap mengikat. Bila timeline HTML sudah memuat caption dan identitas penerbit, satu request profil cukup untuk sampel yang tersedia. Detail API hanya diminta bila informasi tersebut tidak tersedia/valid; jalur itu memerlukan bootstrap anonim dan request detail per postingan. Pagination dapat menambah request. Penelusuran profil dari hasil riset web memperoleh budget metadata sesuai batas postingan, selama budget sesi masih tersedia.

## Yang disimpan dan diekstrak

| Data | Letak dalam JSON |
|---|---|
| Username dan nama tampilan yang diamati | `candidates`, dengan keputusan awal `PENDING` |
| Caption, URL, waktu publikasi, username dan ID penerbit | `social_posts[]` |
| Mention, hashtag, label eksplisit `Lokasi:`, `Location:`, `Kegiatan:`, `Acara:`, `Event:`, `Organisasi:`, `Organization:` | `social_posts[].clues`, dengan kutipan asli dan `UNVERIFIED` |
| ID bukti, URL sumber, URL request, waktu observasi, hash respons, aturan parser | `social_posts[].sources` dan evidence `instagram_post` |
| Profil sumber, batas sampel, jumlah terkumpul, alasan berhenti, status per tahap, receipt request | Evidence `instagram_collection`; `value` adalah JSON yang diserialisasi |
| Jejak akses dan status sesi | `search.access`, `search.events`, `search.status` |

`social_posts` dibangun ulang dari evidence saat normalisasi. Mengedit tampilan turunannya tidak mengubah bukti sumber. Spasi, baris baru, emoji dan huruf besar caption dipertahankan setelah normalisasi Unicode NFC. Caption `null` berarti sumber secara eksplisit mengembalikan tanpa caption; field yang hilang dianggap kegagalan parser.

Timeline HTML tidak selalu menyediakan waktu publikasi. Dalam kondisi itu `published_at` kosong, `timestamp_status: "UNAVAILABLE"`, dan CLI menampilkan `Published: unknown (not provided by source)`. Waktu observasi tidak dipakai sebagai pengganti waktu unggahan. Caption yang tersedia tetap disimpan tanpa request tambahan hanya untuk melengkapi tanggal.

Postingan hanya dihubungkan ke profil bila username **dan ID akun penerbit** cocok dengan metadata profil. ID `pk` dan ID web `id` merupakan dua jenis ID berbeda: `publisher_id_type` mencatat jenisnya dan pemeriksaan membandingkan ID dengan jenis yang sama. Metadata yang hilang atau berbeda dapat menghentikan pengambilan dengan status gagal/parsial. Ini dapat mengecualikan postingan kolaborasi; belum ada dukungan model kepemilikan multiakun.

Bukti timeline memakai aturan `instagram-timeline-v1`. URL sumber tetap URL postingan, sedangkan `request_url` dan hash menunjuk respons profil atau halaman pagination yang benar-benar menyediakan caption. Detail API memakai `instagram-post-v1`. Postingan dari dua respons tidak diberi hash respons yang sama secara sembarangan.

Mention bukan bukti bahwa dua orang bertemu. Label lokasi bukan bukti lokasi terkini. Caption tidak otomatis menjadi employer, lokasi kandidat, hubungan antarmanusia, atau input PES. Identitas yang dikonfirmasi operator juga tidak memverifikasi isi caption. `OBSERVED` pada evidence berarti konten diamati dari sumber; klaimnya tetap `UNVERIFIED`.

Hash adalah sidik jari respons yang dibaca, bukan bukti kebenaran caption atau tanda tangan Instagram. HTML/JSON respons HTTP lengkap tidak disimpan; ini belum merupakan arsip respons yang dapat diputar ulang. Kutipan caption, metadata terpilih, dan hash tersimpan dalam case.

## Review dan membaca hasil

```bash
./bin/pmiyc next -input data/instagram-search.json
./bin/pmiyc challenge -input data/instagram-search.json \
  -out data/instagram-reviewed.json
./bin/pmiyc profile -input data/instagram-reviewed.json \
  -out data/instagram-profile.json

# Dengan jq bila tersedia:
jq '.social_posts // []' data/instagram-search.json
jq '.evidence[] | select(.type == "instagram_collection") | .value | fromjson' \
  data/instagram-search.json
```

Kartu review menampilkan caption dan provenance. Laporan profil hanya memasukkan postingan dari profil yang dikonfirmasi; klaim caption tetap belum terverifikasi. JSON diagnostik panjang tidak dicetak seluruhnya ke terminal.

## Keandalan dan arti status

- Transport tetap memvalidasi TLS dan membatasi koneksi ke alamat publik; HTTP/2 diaktifkan pada adapter Instagram.
- Budget seluruh sesi dan budget lokal adapter sama-sama diperiksa. Jarak minimum request Instagram tiga detik; waktu adapter maksimal 75 detik atau deadline sesi yang lebih pendek. Respons dibatasi 8 MiB.
- Cookie dan token bootstrap anonim hanya berada di memori. Tidak mengimpor sesi login atau mengunduh media.
- Tidak ada retry otomatis Instagram. HTTP 429 mengikuti `Retry-After`, atau cooldown 15 menit jika tidak diberikan. HTTP 401/403 juga memicu cooldown. Status cooldown dibagi melalui `access.state_dir`; default `data/access-state` tetap aktif untuk Instagram bila config lama tidak memiliki `access`.
- `PRIVATE_PROFILE_NOT_REQUESTED`: metadata profil berhasil, postingan privat tidak diminta.
- `EMPTY`: timeline secara eksplisit kosong. Timeline hilang, login/challenge, atau perubahan format bukan `EMPTY`.
- `sample_limit`: batas sampel tercapai. `source_end`: sumber menyatakan halaman terakhir; tidak membuktikan seluruh riwayat akun tersedia secara anonim.
- Sesi bisa `PARTIAL` walaupun sampel berhasil, karena cakupan akun tidak dinyatakan lengkap. Periksa status profil dan postingan secara terpisah.
- Jika profil berhasil tetapi detail postingan gagal, profil dan postingan yang sudah diperoleh tetap disimpan. Cooldown yang mencegah akses tidak dihitung sebagai request HTTP terkirim.

## Validasi integrasi

Pengujian otomatis mencakup 18 fixture caption pembanding Python; Unicode/null; pagination; publisher mismatch; profil privat; batas respons; budget; cooldown lintas sesi; tanpa retry 503/429; dispatch web/URL; provenance per postingan; label berbasis kutipan; pemisahan review identitas dari klaim; stabilitas normalisasi; dan tidak bertambahnya PES karena caption/diagnostik.

Regresi tambahan memastikan caption timeline hanya memerlukan satu request, waktu yang tidak tersedia tidak dibuat-buat, ID web tidak tertukar dengan ID `pk`, publisher mismatch tidak memicu request detail, serta bukti pagination memakai hash responsnya sendiri.

```bash
go test ./...
go vet ./...
go test -race ./internal/instagram ./internal/collector ./internal/search ./cmd/pmiyc
```

Run integrasi awal: 531 tes/subtes lulus pada 14 paket yang memiliki tes. Setelah perbaikan caption: **538 tes/subtes lulus**, tanpa kegagalan; `go vet` dan pemeriksaan race juga lulus. Diagnostik jaringan bersifat opt-in dan dilewati pada suite biasa. Log perbaikan caption disimpan terpisah di `data/instagram-caption-fix-tests-20261030.jsonl` dan `data/instagram-caption-fix-race-20261030.txt`. Binary WSL dibuat sebagai `bin/pmiyc`; executable sebelum integrasi disimpan sebagai `bin/pmiyc-before-instagram-20261030`.

Uji langsung 30 Oktober (WIB), satu akun milik pengguna:

| Run | Hasil |
|---|---|
| Integrasi awal, sebelum negosiasi HTTP/2 | Profil HTTP 302; berhenti, tidak diklaim berhasil |
| Prototipe Go yang tidak diubah, kontrol | Profil dan satu caption berhasil, 3 request, sekitar 6,5 detik |
| Integrasi dengan HTTP/2 | Profil dan bootstrap HTTP 200; detail postingan HTTP 429, 3 request; profil tersimpan dan cooldown aktif |
| CLI final selama cooldown | `BLOCKED`, 0 request terkirim; `search.access` mencatat `DEFERRED` dan waktu cooldown |
| CLI final setelah cooldown, sekitar 17:33 WIB | Profil dan bootstrap HTTP 200; detail postingan kembali HTTP 429. Ketiganya memakai HTTP/2. Profil tersimpan, 0 caption; cooldown kembali aktif |
| Perbaikan pembacaan timeline, sekitar 17:53 WIB | Satu respons profil HTTP 200 menghasilkan caption “[caption akun uji disamarkan]”; status postingan `SUCCESS`, `source_end`, 1 request, sekitar 0,94 detik waktu collector. Tidak ada request detail API |
| Verifikasi `bin/pmiyc` final, sekitar 17:57 WIB | Caption dan URL yang sama berhasil lagi; status postingan `SUCCESS`, 1 request. Artefak: `data/instagram-caption-final-20261030.json` dan `.txt` |

Artefak hasil perbaikan: `data/instagram-caption-fixed-20261030.json` dan `.txt`. Caption serta URL postingan identik dengan hasil kontrol prototipe `../instagram-go-prototype/results/integration-control-20261030.json`. Sumber timeline tidak memberikan timestamp yang tersedia dalam hasil detail API prototipe; perbedaan ini dipertahankan secara eksplisit.

Artefak sebelum perbaikan: `data/instagram-integration-own-20261030.json`, `data/instagram-integration-http2-20261030.json`, `data/instagram-integration-cooldown-20261030.json`, dan `data/instagram-integration-final-20261030.json`. Semua artefak akun berada di direktori lokal yang tidak ditambahkan ke Git.

**Batas kesimpulan:** caption sudah berhasil diperoleh secara langsung melalui collector terintegrasi untuk akun uji tersebut. Penyebab 429 pada endpoint detail belum diisolasi; perbaikan menghilangkan kebutuhan akan request itu ketika caption sudah ada dalam respons profil. Endpoint anonim Instagram tetap dapat berubah atau dibatasi. Satu akun uji bukan bukti akses akan selalu berhasil pada semua akun.
