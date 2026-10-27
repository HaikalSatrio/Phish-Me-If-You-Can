# Keandalan akses publik

Keberhasilan koneksi, tersedianya hasil pencarian, dan kecocokan identitas adalah tiga ukuran berbeda. HTTP 200 belum membuktikan halaman berisi hasil yang dapat dipakai. Kandidat yang ditemukan tetap memerlukan review identitas.

## Menjalankan di WSL

Dari direktori repository, dengan SearXNG lokal berjalan pada port 8888:

```bash
go build -o bin/pmiyc ./cmd/pmiyc
./bin/pmiyc search -source web -name "Simon Willison" \
  -config configs/collector-reliable-v4.json
```

Profil ini bersifat opt-in. Konfigurasi lama tetap tersedia untuk reproduksi baseline. Ia memilih engine Bing dan Yahoo di SearXNG setelah probe lokal; ketersediaannya tetap bergantung pada instance dan layanan asal. Profil `collector-reliable.json`, `v2`, dan `v3` merupakan konfigurasi eksperimen sebelumnya, bukan rekomendasi penggunaan saat ini.

## Perubahan yang tersedia

| Bagian | Perilaku |
|---|---|
| Pemilihan engine | Saat `searxng_engines` diisi, request tidak lagi menyertakan `categories=general`. Ini mencegah SearXNG menambahkan engine kategori umum di luar pilihan. |
| DNS DuckDuckGo | Profil memakai DNS over HTTPS Cloudflare hanya untuk `html.duckduckgo.com`. Sertifikat tetap diperiksa dan tujuan nonpublik tetap ditolak. Host lain memakai resolver sistem. |
| TLS Bing | `classical_tls_hosts` membatasi kurva pertukaran kunci ke X25519/P-256/P-384 hanya untuk `www.bing.com`. TLS minimum 1.2, pemeriksaan sertifikat/hostname, dan pemeriksaan alamat tetap aktif. Profil ini tidak menawarkan pertukaran kunci post-quantum pada host tersebut. |
| Jarak request | Jadwal per host dibagi antarproses yang menggunakan `access.state_dir` sama: minimum 1 detik, atau 6 detik untuk pencarian. Request memesan slot, lalu jarak antar-start tetap diperiksa agar proses cepat tidak terus mengambil giliran proses lain. |
| Retry | Maksimum satu retry pada profil ini untuk timeout atau HTTP 502/503/504, mengikuti batas waktu dan budget. Kegagalan sertifikat tidak diulang. |
| Pembatasan | HTTP 429 mengikuti `Retry-After`, atau menunggu 15 menit jika tidak ada. HTTP 401/403 serta halaman challenge/login juga menimbulkan cooldown. |
| Cooldown pendek | Profil ini mengizinkan total tunggu cooldown sampai 5 detik per pemanggilan gate, jika masih muat dalam deadline sesi. Perpanjangan cooldown tidak mereset jatah tunggu. Cooldown lebih panjang dicatat sebagai request yang ditunda. |
| Engine tersuspensi | SearXNG mengelola cooldown masing-masing engine. Kegagalan satu engine tidak menutup akses ke seluruh layanan lokal. |
| Audit | `search.access` menyimpan host, hasil koneksi, status HTTP, durasi, dan alasan penundaan. Query dan kredensial tidak disalin ke catatan transport ini. |

`access.state_dir` relatif terhadap direktori kerja. Gunakan direktori yang sama untuk semua proses yang harus berbagi batas akses. State tidak mengoordinasikan browser, aplikasi ponsel, perangkat lain, atau request engine di dalam SearXNG; pembatasan SearXNG tetap berlaku sendiri.

Jika proses berhenti paksa saat menulis state, berkas `.lock` bisa tertinggal. Tool berhenti mengakses host itu, bukan mengabaikan lock. Periksa bahwa tidak ada proses PMIYC aktif sebelum membersihkan lock yang benar-benar tertinggal. Jangan menghapus berkas state untuk menghindari cooldown.

## Membaca hasil

- `RESPONSE`: menerima respons HTTP; periksa status dan hasil parser terpisah.
- `NETWORK_ERROR`: koneksi/request gagal, termasuk DNS, TLS, dan timeout.
- `DEFERRED`: request tidak dikirim karena cooldown, deadline, pembatalan, atau budget. **Bukan akses sukses.**
- `STATE_ERROR`: gagal menyimpan cooldown; periksa direktori state.

Budget mencakup percobaan request konten, termasuk retry. Lookup DNS adalah request infrastruktur di luar hitungan tersebut. Browser Google memiliki mekanisme dan penghitungan terpisah. Cache konten tidak digunakan untuk menghasilkan angka pengujian ini.

Instagram dapat tetap menolak akses anonim. Hasil indeks tentang Instagram bukan bukti bahwa halaman Instagram berhasil diambil. Jika akses langsung dibutuhkan, evaluasi jalur resmi yang sesuai izin dan cakupan studi; mengganti scraper saja tidak menjamin 429 hilang.

## Uji ulang

```bash
python3 scripts/benchmark_consistency.py \
  --binary bin/pmiyc --config configs/collector-reliable-v4.json \
  --rounds 3 --out data/access-run-baru
python3 scripts/report_access_reliability.py \
  --run data/access-run-baru --out data/access-run-baru/access-summary.json
python3 scripts/report_consistency.py \
  --run data/access-run-baru --out data/access-run-baru/consistency-summary.json
```

Runner memakai delapan nama publik, dua pekerja, jeda awal minimal enam detik, dan kontrol API GitHub untuk enam username yang sudah diketahui. Gunakan direktori baru untuk eksperimen dengan binary/config berbeda. Jangan menafsirkan pengulangan terhadap delapan orang sebagai sampel populasi besar.

## Tools yang ditelaah

| Tool | Keputusan untuk PMIYC |
|---|---|
| [SearXNG](https://docs.searxng.org/dev/search_api.html) | Dipakai; memperbaiki parameter engine pada integrasi yang sudah ada. |
| [go-retryablehttp](https://github.com/hashicorp/go-retryablehttp) | Mendukung retry/backoff. Tidak ditambahkan karena PMIYC memerlukan penghitungan setiap percobaan, state antarproses, dan guard tujuan publik yang sudah ada. |
| [Resty](https://github.com/go-resty/resty) | Alternatif HTTP client; migrasi penuh belum diperlukan untuk masalah yang ditemukan. |
| [Colly](https://github.com/gocolly/colly) | Framework crawler; tidak memperbaiki sendiri DNS keliru atau pembatasan penyedia. |
| [Instaloader](https://instaloader.github.io/troubleshooting.html) | Dokumentasinya juga menjelaskan 429 dan rate controller. Tidak dipasang sebagai klaim penyelesaian akses Instagram. |
| [Cloudflare DNS over HTTPS](https://developers.cloudflare.com/1.1.1.1/encryption/dns-over-https/make-api-requests/dns-json/) | Dipakai secara terbatas untuk mengatasi perbedaan resolusi DNS DuckDuckGo yang teramati. |

Tidak ada dependency baru yang dipasang. Implementasi menggunakan pustaka standar Go dan mempertahankan pemeriksaan TLS serta alamat tujuan.

Go menjelaskan dukungan konfigurasi kurva dan perubahan default hybrid key exchange dalam [dokumentasi TLS](https://go.dev/pkg/crypto/tls/?m=old) dan [rilis Go 1.26](https://go.dev/doc/go1.26). Uji lokal menemukan handshake Bing macet dengan default toolchain yang dipakai, sedangkan profil klasik merespons. Hasil ini menunjukkan masalah kompatibilitas pada jalur yang diuji; belum mengidentifikasi perangkat jaringan atau penyebab internal penyedia secara pasti.
