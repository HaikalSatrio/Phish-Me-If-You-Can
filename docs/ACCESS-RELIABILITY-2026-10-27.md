# Evaluasi perbaikan akses — 27 Oktober 2026

**Hasil akhir:** 66 sesi selesai dalam tiga putaran, 27 Oktober 2026 sekitar 11.24–11.41 WIB. Perbaikan meningkatkan tersedianya kandidat dan akses Bing/SearXNG, tetapi belum menyelesaikan challenge DuckDuckGo, 429 Instagram, atau latensi. Seluruh sesi web tetap berstatus `PARTIAL`, dan identitas kandidat tetap `PENDING`.

## Hasil final

| Ukuran | Baseline | Final |
|---|---:|---:|
| Sesi nama saja dengan kandidat | 20/24 | 24/24 |
| Sesi nama + akun dengan kandidat | 24/24 | 24/24 |
| Kontrol GitHub API dengan profil | 18/18 | 18/18 |
| URL referensi ditemukan, nama saja | 29/57 | 49/57 |
| URL referensi ditemukan, nama + akun | 48/57 | 54/57 |
| Kemiripan himpunan URL antarputaran, nama saja | 0,532 | 0,913 |
| Kemiripan himpunan URL antarputaran, nama + akun | 0,634 | 0,917 |
| Median durasi nama saja | 8,6 detik | 24,1 detik |
| Median durasi nama + akun | 21,3 detik | 46,3 detik |

**Keandalan akses tidak sama dengan kecepatan atau akurasi identitas.** Latensi memburuk dibanding baseline karena pengaturan jarak request, antrian, retry, dan sumber yang diakses berbeda. Durasi maksimum final 44,2 detik untuk nama saja dan 84,5 detik untuk nama + akun. Seluruh sesi tetap dalam budget 20 percobaan request. Evaluasi final belum mengaudit ketepatan identitas hasil baru; konsistensi URL tinggi juga dapat mengulang hasil yang salah.

| Sumber | Observasi final | Interpretasi |
|---|---|---|
| Bing langsung | 60 HTTP 200 dari 60 percobaan; nol timeout | Perbaikan transport bertahan pada pengujian ini. Hanya 20 dari 60 operasi pencarian menghasilkan tautan yang lolos filter; akses pulih belum berarti relevansi baik. |
| SearXNG lokal | 150 HTTP 200; 148 respons tanpa error upstream, 2 dengan error upstream | Layanan lokal dan fallback engine berfungsi; belum berarti seluruh engine selalu sehat. |
| DuckDuckGo | 1 HTTP 202/challenge; 34 penundaan cooldown; tidak ada HTTP 200 pada iterasi final | TLS berhasil sampai menerima challenge, tetapi sumber tidak menyediakan hasil saat evaluasi final. Keberhasilan HTTP 200 pada iterasi sebelumnya tidak boleh dianggap sebagai ketersediaan final. |
| Instagram langsung | 1 redirect, 1 HTTP 429, 11 penundaan; seluruh 6 operasi anchor Instagram `BLOCKED` | Belum berhasil mengambil profil langsung. Hasil indeks Instagram adalah bukti dari mesin pencarian, bukan keberhasilan akses Instagram. |
| Halaman tujuan lain | 8 kegagalan jaringan pada TikTok/Equilar; pembatasan lain masih ada | Fallback menjaga proses berjalan, tetapi tidak menghilangkan semua kegagalan sumber. |

Total final: **433 percobaan request konten**. Penundaan tidak masuk hitungan tersebut. Kegagalan transport di halaman lain dan rate limit tetap dicatat, bukan diubah menjadi sukses.

## Apa yang diperbaiki

| Masalah | Temuan dan perbaikan |
|---|---|
| DuckDuckGo gagal TLS | Resolver sistem dan resolver HTTPS memberikan alamat berbeda. Profil baru memakai DNS over HTTPS hanya untuk host pencarian DuckDuckGo, dengan pemeriksaan sertifikat dan alamat tujuan tetap aktif. Iterasi 2 memperoleh 53 respons HTTP 200 tanpa kegagalan jaringan pada host ini. Challenge masih bisa muncul. |
| Bing timeout | Trace menunjukkan TCP terhubung, tetapi handshake TLS tidak selesai. Dua probe default timeout; dua probe dengan kurva TLS klasik memperoleh HTTP 200 dalam 306/361 ms. Iterasi 3 kemudian memperoleh 23 HTTP 200 dari Bing. Profil kompatibilitas dibatasi ke `www.bing.com`. |
| Instagram 429 | Belum teratasi sebagai akses langsung. Request dibatasi dengan `Retry-After`/cooldown persisten; sumber indeks dan situs publik lain tetap bisa dipakai. Penundaan tidak dihitung sebagai akses berhasil. |
| Engine SearXNG suspended | Ditemukan parameter `engines` dikirim bersama `categories=general`, sehingga engine kategori umum ikut ditambahkan. Kini pilihan engine benar-benar dibatasi. Suspensi tidak dihapus; engine alternatif diuji dan profil final memakai Bing + Yahoo. |

Temuan Bing menunjukkan kompatibilitas TLS pada jalur yang diuji. Tidak cukup bukti untuk menyalahkan perangkat jaringan tertentu atau menyatakan semua timeout Bing di semua jaringan memiliki penyebab yang sama. Uji ulang binary lama sebelum perubahan juga pernah menerima feed, sehingga kondisi jaringan memang berubah.

## Iterasi dan keterbukaan hasil

| Iterasi | Sesi dengan checkpoint | Hasil yang menentukan langkah berikutnya |
|---|---:|---|
| Baseline | 66 | DDG 26 gagal; Bing 26 timeout; Instagram anchor 6 diblokir. |
| 1 | 22 | DNS HTTPS membantu DDG; Bing masih gagal. Ditemukan bug pemilihan engine. |
| 2 | 27, parsial | DDG 53 HTTP 200, Bing 15 kegagalan jaringan. Dihentikan setelah diagnosis TLS, sebelum tiga putaran selesai. |
| 3 | 12, parsial | Bing 23 HTTP 200. Ditemukan regresi cooldown yang menutup seluruh SearXNG dan dapat menghabiskan deadline. Iterasi dihentikan untuk memperbaikinya. |
| 4 | 43, parsial | Bing 40 HTTP 200; SearXNG 97 HTTP 200. Semua 32 sesi web memiliki kandidat, tetapi ada starvation antrian sampai sekitar 83 detik; diperbaiki sebelum evaluasi berikutnya. |
| 5 | 66, lengkap | Tiga putaran dengan antrian slot: 48/48 sesi web memiliki kandidat, 18/18 kontrol API berhasil; Bing 60 HTTP 200 dan SearXNG 150 HTTP 200. Batas sumber dan latensi dicatat di atas. |

Perbaikan regresi: kegagalan upstream tidak lagi menutup seluruh layanan SearXNG; jatah tunggu cooldown tidak dimulai ulang ketika diperpanjang oleh proses lain. Profil final membatasi tunggu cooldown menjadi 5 detik. Snapshot pembatalan tanpa checkpoint disimpan tetapi dikecualikan dari ringkasan.

Iterasi 4 kemudian menemukan masalah penjadwalan: satu proses menunggu sekitar 83 detik karena proses lain terus memperoleh giliran pada host yang sama. Perbaikan berikutnya memesan slot request sekali, kemudian memeriksa jarak antar-start dan cooldown baru sebelum mengirim. Tes antrian bersamaan dijalankan ulang lima kali dengan race detector. `Retry-After: 0` tidak lagi membuat cooldown kosong di disk.

## Metode

- Delapan nama publik yang sama dengan baseline: enam pengembang perangkat lunak dan dua figur hiburan.
- Setiap putaran: 8 pencarian nama, 8 nama + akun yang diketahui, 6 kontrol GitHub API berdasarkan username.
- Dua pekerja, jeda awal minimal 6 detik; maksimum 20 percobaan request konten, 5 kandidat, timeout request 15 detik dan sesi 90 detik.
- Binary, konfigurasi, corpus, dan runner dicatat dengan SHA-256. Snapshot yang berubah ditolak oleh reporter.
- State cooldown dibawa antariterasi. Tidak menghapus suspensi SearXNG, memakai cache konten sebagai hasil baru, mengubah entity score, atau mengonfirmasi identitas otomatis.
- Pengujian dilakukan berurutan dengan kondisi penyedia berubah; bukan eksperimen acak untuk mengisolasi efek setiap perubahan. Ada jeda beberapa jam antara iterasi 1 dan 2.

HTTP 200 mengukur respons, bukan validitas isi. Status operasi mencatat apakah parser memperoleh bukti yang dapat dipakai; status kandidat/identitas dinilai terpisah. `DEFERRED` berarti tidak dikirim. Hitungan request adalah percobaan client konten; kegagalan jaringan tidak membuktikan request mencapai penyedia. Lookup DNS berada di luar budget konten.

## Engine alternatif dan tools

Satu query per engine, jeda 12 detik: Mwmbl merespons tanpa hasil, Qwant menolak akses, Yahoo memberi 7 hasil tanpa error engine. Ini alasan memilih Yahoo untuk evaluasi lanjutan, bukan jaminan ketersediaan jangka panjang.

[go-retryablehttp](https://github.com/hashicorp/go-retryablehttp), [Resty](https://github.com/go-resty/resty), [Colly](https://github.com/gocolly/colly), dan [Instaloader](https://instaloader.github.io/troubleshooting.html) ditelaah. Tidak ada dependency baru yang diperlukan: fungsi retry, pengaturan jarak request, dan cooldown diintegrasikan ke transport yang sudah memiliki guard alamat publik. Instaloader sendiri mendokumentasikan 429; memasangnya tidak membuktikan Instagram menjadi dapat diakses.

Rujukan implementasi: [SearXNG Search API](https://docs.searxng.org/dev/search_api.html), [Cloudflare DNS JSON](https://developers.cloudflare.com/1.1.1.1/encryption/dns-over-https/make-api-requests/dns-json/), dan [perubahan TLS Go 1.26](https://go.dev/doc/go1.26). Profil TLS Bing mempertahankan verifikasi sertifikat/hostname dan minimum TLS 1.2, tetapi tidak menawarkan pertukaran kunci post-quantum untuk host itu.

## Verifikasi kode dan artefak

`go test ./...`, `go vet ./...`, tes race untuk collector/search, serta tes reporter konsistensi lulus. Tes regresi mencakup budget retry, `Retry-After`, cooldown antarproses, perpanjangan cooldown, pemilihan engine, kegagalan satu engine tanpa memblokir layanan, dan penolakan sertifikat tidak tepercaya pada mode TLS kompatibel.

- Diagnosis: [JSON probe](../benchmarks/results/2026-10-27-access-diagnostics.json).
- Final: [akses dan status sumber](../benchmarks/results/2026-10-27-access-final.json), [cakupan dan konsistensi](../benchmarks/results/2026-10-27-access-final-consistency.json).
- Iterasi sebelumnya: [1](../benchmarks/results/2026-10-27-access-iteration-1.json), [2 parsial](../benchmarks/results/2026-10-27-access-iteration-2.json), [3 parsial](../benchmarks/results/2026-10-27-access-iteration-3.json), [4 parsial](../benchmarks/results/2026-10-27-access-iteration-4.json).
- Baseline: [hasil konsistensi](../benchmarks/results/2026-10-27-consistency.json).
- Penggunaan WSL dan pengaturan: [panduan akses](ACCESS-RELIABILITY.md).
- Snapshot mentah lokal: `data/access-repair-20261027/`; tidak masuk Git.

Perubahan utama ada di `access.go`, `dns_https.go`, `public_http.go`, `searxng.go`, dan `search.go`. `model.go` menambahkan jejak transport terpisah dari coverage/PES. Konfigurasi final berada di `configs/collector-reliable-v4.json` dan bersifat opt-in.

Binary WSL `bin/pmiyc` identik dengan `bin/pmiyc-access-v6` yang diuji, SHA-256 `6ae4cf1ed9526eb002662cf919bb2e5aebb1ef6e4e6ed8ad31547df1638f8f78`. Versi sebelumnya disimpan di `bin/pmiyc-before-access-20261027`. Perubahan belum di-commit.

## Yang masih perlu dikerjakan

1. Kurangi latensi dengan evaluasi terpisah atas jumlah query dan kebijakan retry; jangan mengurangi jeda akses tanpa mengukur dampak pembatasan.
2. Audit relevansi/identitas hasil baru. Perbaikan akses tidak mengkalibrasi entity score atau memvalidasi klaim profil.
3. Jika pengambilan Instagram langsung merupakan syarat, tetapkan jalur akses resmi/berizin sesuai cakupan penelitian. Akses anonim saat ini belum memenuhi syarat itu.
4. Uji pada hari dan kondisi jaringan berbeda serta nama yang tidak dipakai saat pengembangan. Pilot ini belum membuktikan ketersediaan jangka panjang.
