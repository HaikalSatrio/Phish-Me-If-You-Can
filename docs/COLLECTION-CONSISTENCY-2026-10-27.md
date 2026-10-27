# Diagnosis konsistensi pengumpulan — 27 Oktober 2026

**Prioritas pertama: keandalan jalur pencarian dan pengaturan request lintas sesi.** Pengulangan memperlihatkan penurunan hasil ketika engine pencarian mengalami pembatasan. Akses API GitHub tetap berhasil. Setelah itu, perbaiki pemetaan data yang sudah terbaca menjadi field terstruktur dan penyaringan kandidat.

Ini hasil pengujian live baru di WSL, bukan rangkuman benchmark sebelumnya. Kode collector tidak diubah selama pengujian.

## Lingkup dan hasil empat faktor

- Commit: `6a918c6`; binary baru dibangun dengan Go 1.26.0 di Ubuntu/WSL2.
- Waktu: 27 Oktober 2026, sekitar 01:59–02:08 WIB (timestamp artefak menggunakan UTC).
- Delapan subjek publik: Simon Willison, Guido van Rossum, Linus Torvalds, Taylor Swift, Maudy Ayunda, Rich Harris, Kent C. Dodds, Addy Osmani.
- 24 pencarian nama + 24 pencarian nama dengan petunjuk akun + 18 kontrol API GitHub = **66 sesi**, **425 request yang dihitung collector**. Angka request tidak mencakup fan-out engine di belakang SearXNG, preflight, pembentukan referensi, atau probe tambahan.
- Tiga putaran dalam satu sesi; dua worker pencarian, jarak awal minimal enam detik; kontrol API berjalan serial.
- Konfigurasi asli: lima kandidat, 20 request, timeout request 15 detik, deadline sesi 90 detik. Tanpa enrichment LLM.

| Faktor | Hasil | Penilaian |
| --- | --- | --- |
| Keandalan akses | Nama saja: 4/24 sesi FAILED dan kosong. Dengan akun: 24/24 menghasilkan kandidat, tetapi semuanya PARTIAL. API GitHub: 18/18 berhasil mengambil profil. | **Masalah prioritas pertama pada jalur pencarian dan fallback.** |
| Ketepatan subjek | Audit 56 pasangan subjek/URL unik: 50 relevan, 2 salah orang, 2 halaman umum tidak relevan, 2 belum pasti. Semua kandidat tetap PENDING. | Penyaringan belum cukup; tidak terjadi penerimaan identitas otomatis. |
| Kelengkapan bukti | URL referensi yang muncul sebagai kandidat: nama 29/57; dengan akun 48/57. Pada enam profil GitHub, mode akun mempertahankan 63/63 nilai referensi di observasi teks, tetapi hanya 18/63 menjadi field observed bertipe; API langsung 63/63. | Ada kekurangan discovery **dan** pemetaan field. Data tidak selalu hilang dari halaman. |
| Kestabilan | Rata-rata Jaccard URL antarputaran: nama 0,532; akun 0,634; kontrol API 1,000. Jaccard observasi: 0,474; 0,552; 1,000. | Pengulangan web belum stabil dalam lingkungan ini. |

Jaccard = irisan dibagi gabungan himpunan hasil. Nilai 1 berarti himpunannya sama, bukan benar. Pasangan dua hasil kosong tidak diberi nilai 1: dikeluarkan sebagai tidak terdefinisi. Ada 23 pasangan yang dinilai untuk nama, 24 untuk akun, dan 18 untuk API. Perbandingan observasi menggunakan URL kandidat, tipe, dan nilai yang dinormalisasi; perubahan teks dinamis juga menurunkannya.

## 1. Keandalan: kegagalan ditemukan di sumber yang mana?

| Jalur/operasi | Hasil tercatat |
| --- | --- |
| SearXNG | 101 operasi SUCCESS, 14 FAILED. SUCCESS dapat berarti respons valid dengan nol hasil relevan; bukan jaminan semua engine sehat. |
| DuckDuckGo langsung | 0/26 operasi berhasil; 24 kegagalan TLS dan dua timeout. |
| Bing langsung | 0/26 operasi berhasil; timeout. |
| Anchor Instagram | 0/6 berhasil; HTTP 429. Cuplikan indeks tetap bisa ditemukan. |
| Anchor halaman GitHub | 18/18 berhasil. |
| Kontrol API GitHub | 18/18 berhasil; median 0,735 detik. |
| Pengambilan halaman hasil web | 129/151 operasi berhasil; 13 BLOCKED dan 9 FAILED. |

Ini jumlah **event operasi**, bukan pengukuran availability per HTTP request. Semua kegagalan tetap disimpan.

Probe tambahan menemukan:

- Awal sesi, hasil SearXNG tersedia melalui Brave dan Google CSE; DuckDuckGo sudah melaporkan masalah SSL, Wikipedia rate limit.
- Sekitar 02:04 WIB, satu probe menghasilkan nol hasil; Brave, Google CSE, dan Wikipedia berstatus suspended/rate limit, DuckDuckGo suspended karena SSL, Wikidata timeout.
- Pemeriksaan TLS langsung ke `html.duckduckgo.com` gagal dengan **hostname mismatch**. Ini bukan bukti bahwa memasang CA tambahan akan menyelesaikannya. Jalur DNS/jaringan harus diperiksa; verifikasi TLS tetap aktif.
- TLS Bing berhasil dalam probe, sedangkan request pencarian dari collector timeout. Jadi akar masalah Bing belum terisolasi; jangan menyamakannya dengan masalah TLS DuckDuckGo.

Jumlah kandidat nama saja turun dari **40 → 35 → 17** pada putaran 1–3. Sesi kosong naik **0 → 1 → 3**. Kegagalan total terjadi pada Addy (putaran 2 dan 3), Kent (3), dan Rich (3). Kontrol API tidak mengalami penurunan.

Pembatasan engine berkorelasi dengan degradasi hasil. Pengujian ini belum memisahkan kontribusi beban benchmark, pengguna lain pada instance yang sama, dan kebijakan provider. Enam detik antarawal proses tidak menjamin jarak antar-request pada setiap engine: satu sesi dapat mengirim beberapa query dan SearXNG meneruskannya ke beberapa engine.

## 2. Ketepatan: kandidat masih dapat salah orang

- Query **Rich Harris** memasukkan Wikipedia dan IMDb **Judith Rich Harris**.
- Penelusuran Guido memasukkan `computerhistory.org/about`; penelusuran Linus memasukkan kategori buku `goodreads.com/genres/biography`.
- Dua hasil dengan nama/handle yang cocok belum mempunyai konteks yang cukup untuk memutuskan orang yang dimaksud.

Audit adalah review tunggal oleh asisten terhadap judul, cuplikan, dan konteks profil yang tersimpan. Label RELEVANT hanya berarti halaman membahas subjek; bukan verifikasi pemilik akun atau seluruh isi halaman. Angka 50/56 tidak boleh dipublikasikan sebagai akurasi identitas PMIYC. Hasil kosong juga tidak masuk denominator audit ini.

Kode terkait: `internal/collector/person_results.go`, `research_clues.go`, dan `research.go`. Pemilihan tautan “about/biography” perlu memeriksa konteks personal; nama sebagai bagian dari nama yang lebih panjang membutuhkan penandaan ambiguitas.

## 3. Kelengkapan: pisahkan discovery, pembacaan, dan pemetaan

| Ukuran | Nama saja | Dengan akun | API GitHub langsung |
| --- | ---: | ---: | ---: |
| URL referensi ditemukan | 29/57 (50,9%) | 48/57 (84,2%) | Tidak dibandingkan: scope hanya akun yang diberikan |
| URL referensi selain akun petunjuk | 21/33 (63,6%) | 25/33 (75,8%) | Tidak berlaku |
| Nilai referensi GitHub tampak dalam observasi sumber yang sama | 8/63 | 63/63 | 63/63 |
| Nilai referensi menjadi field observed sesuai tipenya | 3/63 | 18/63 | 63/63 |

URL referensi terdiri dari 19 URL yang ditetapkan sebelum pengumpulan, diulang tiga kali. Pencocokan ketat pada URL kandidat setelah normalisasi dasar; URL yang hanya tertulis dalam tautan halaman tidak dihitung. Ini cakupan terhadap daftar terbatas, bukan recall seluruh internet. Redirect seperti situs Guido ke host baru dan halaman `/about` sebagai pengganti homepage bisa membuat ukuran URL ketat meremehkan cakupan sumber yang sebenarnya.

Referensi field adalah 21 nilai nonkosong dari enam respons API GitHub yang diambil sebelum matriks: nama, company, website, lokasi; diulang tiga kali. Pemeriksaan hanya pada observasi milik kandidat GitHub yang sama. Field kosong dikeluarkan, bukan dianggap salah. Kecocokan teks adalah proxy retensi nilai, bukan penilaian semantik.

Mode akun hanya memetakan nama menjadi tipe yang sesuai pada pemeriksaan ini; company/website/lokasi masih ada dalam teks. Jalur API khusus menyediakan field tersebut secara terstruktur. Usulan hubungan hasil ekstraksi diperiksa terpisah oleh aplikasi dan tidak otomatis menjadi fakta observed atau PES.

Nilai API merupakan klaim sumber, bukan kebenaran terkini. Salah satu profil mempunyai company dan bio yang menyebut organisasi berbeda; referensi ini tidak menyelesaikan konflik tersebut.

**33/48 sesi web mencapai batas lima kandidat**, sementara **0/48 menghabiskan 20 request**. Ini indikasi batas kandidat dapat membatasi pengumpulan lebih awal. Besarnya manfaat menaikkan batas belum diuji; jangan langsung menaikkan batas saat sumber sedang terkena rate limit.

## Fokus pengerjaan berikutnya

| Urutan | Pekerjaan | Bukti selesai yang diperlukan |
| --- | --- | --- |
| 1 | Periksa DNS/rute DuckDuckGo; tandai provider yang gagal TLS sebagai tidak tersedia. Isolasi timeout Bing. | Probe TLS valid atau alasan unavailable jelas; tidak mengulangi provider yang diketahui rusak di setiap sesi. |
| 2 | Antrean dan cooldown per provider lintas sesi; hormati Retry-After; hentikan sementara query ke engine suspended. | Pengujian kegagalan buatan membuktikan tidak ada request sebelum cooldown; budget/deadline tetap dipatuhi. |
| 3 | Diagnosis engine SearXNG sebelum run, batasi fan-out dan burst, catat engine aktual. Simpan versi image/config. | Ulangi pilot pada konfigurasi yang dibekukan dan bandingkan kegagalan serta kestabilan per putaran. |
| 4 | Hubungkan adapter API yang sudah ada untuk akun publik yang diketahui, dengan provenance dan konflik field tetap tercatat. | Retensi field mendekati kontrol API pada kasus yang didukung, tanpa menerima identitas otomatis. |
| 5 | Perbaiki penyaringan salah orang/tautan umum; pisahkan batas identitas, URL, dan bukti. | Regression untuk Judith/Rich dan halaman About umum; eksperimen batas dilakukan setelah akses stabil. |

Saran sementara untuk pemakaian: berikan petunjuk akun yang memang diketahui; untuk GitHub gunakan collector API yang tersedia bila sesuai. Jangan mengartikan status PARTIAL atau berkurangnya hasil sebagai turunnya exposure seseorang.

## Batas kesimpulan

- Hanya delapan figur publik, enam di antaranya developer. Tidak mewakili populasi umum atau profil dengan jejak sedikit.
- Empat kasus memakai kembali corpus lama. Ini diagnosis development, bukan final heldout.
- Pengulangan dilakukan dalam sekitar sembilan menit, bukan lintas hari/jaringan.
- API hanya kontrol akses dan retensi field dari enam handle yang diberikan. Bukan bukti API lebih baik untuk pencarian nama atau semua platform.
- Ketepatan identitas sebenarnya serta kelengkapan seluruh jenis bukti PES belum dapat dinilai dari pilot ini.
- Semua output yang ada, termasuk FAILED, dianalisis. Tidak dilakukan retry selektif untuk mengganti hasil gagal.

## Artefak dan menjalankan ulang di WSL

- [Hasil per sesi dan ringkasan](../benchmarks/results/2026-10-27-consistency.json).
- [Audit 56 pasangan subjek/URL](../benchmarks/results/2026-10-27-consistency-audit.json).
- [Probe sumber](../benchmarks/results/2026-10-27-consistency-source-probes.json).
- [Corpus pilot](../fixtures/consistency-pilot-v1.json).
- Snapshot dan log asli: `data/consistency-20261027/` (lokal, diabaikan Git).

```bash
cd /mnt/c/Users/ATHAR/Documents/BISMILLAH-BLACKHAT/OSINTCENG/Phish-Me-If-You-Can
go build -o bin/pmiyc-consistency ./cmd/pmiyc
python3 scripts/benchmark_consistency.py --out data/consistency-new-run
python3 scripts/report_consistency.py --run data/consistency-new-run --out data/consistency-new-run/report.json
```

Gunakan folder baru untuk eksperimen baru. Sebelum mengulang pengujian live, periksa kesehatan/cooldown sumber; default enam detik bukan jaminan bebas rate limit. Reporter menghasilkan template audit URL; output baru tidak otomatis mempunyai label ketepatan. Gunakan `--audit` hanya dengan label untuk pasangan subjek/URL yang sudah direview.

Validasi yang dijalankan: `go test ./internal/collector ./internal/search` lulus; tiga tes metrik Python lulus. Verifier memeriksa 66 pasangan kasus/mode/putaran yang diharapkan serta hash seluruh snapshot. Tes unit yang lulus tidak membantah kegagalan live di atas.
