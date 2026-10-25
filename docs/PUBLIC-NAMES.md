# Nama panggung, nama publik, dan ambiguitas

Pencarian dan enrichment sekarang menyimpan `name_claims`: hubungan nama yang dinyatakan secara eksplisit pada sumber publik. Contohnya, kalimat dengan pola `known professionally as`, `nama panggung`, `bernama asli`, atau `whose real name is`. Fitur ini berjalan tanpa Ollama dan tanpa API berbayar. Pencarian web tetap memerlukan provider yang dapat diakses; konfigurasi bawaan memakai SearXNG lokal dengan fallback publik.

```powershell
.\bin\pmiyc.exe search -source web -name "Lorde" -out data/lorde-search.json
.\bin\pmiyc.exe next -input data/lorde-search.json
.\bin\pmiyc.exe challenge -input data/lorde-search.json -out data/lorde-reviewed.json
.\bin\pmiyc.exe profile -input data/lorde-reviewed.json -out data/lorde-profile.json
```

Snapshot lama dapat diproses ulang tanpa request web:

```powershell
.\bin\pmiyc.exe enrich -input data/hasil-lama.json -out data/hasil-nama.json
```

Setiap klaim membawa alias, nama publik, kandidat asal, ID bukti, kutipan literal, metode, dan status `UNVERIFIED`. Kutipan mempertahankan teks sumber termasuk gelar; gelar pascanama yang dikenali seperti KBE dipisahkan dari nilai nama. Nama hasil ekstraksi tidak mengganti `display_name`, tidak menjadi identitas legal terverifikasi, dan tidak menaikkan PES atau mengonfirmasi kandidat.

Ekstraktor memakai nama alternatif untuk mencari hubungan pekerjaan/profesi hanya dalam dokumen yang secara eksplisit menghubungkan kedua nama. Nama dari satu halaman tidak otomatis ditempelkan ke akun atau halaman lain. `Evidence.aliases` tetap berarti alias ID bukti, bukan alias orang.

Nama yang berbeda untuk alias yang sama tetap dipertahankan dan diberi penanda review. Ini dapat berarti orang berbeda, nama yang dipendekkan, perubahan nama, atau sumber yang tidak konsisten. Misalnya hasil `Sting` dapat memuat musisi dan pegulat, sedangkan `Bono` dapat memuat musisi dan pemain sepak bola. `next` dan `challenge` menampilkan nama beserta sumbernya. Laporan `profile` hanya memakai kandidat yang dikonfirmasi operator, dan tetap menandai variasi nama.

Pencarian web dengan nama saja, tanpa pembatas platform, menyisihkan satu slot kandidat untuk pencarian biografi ketika budget minimal delapan request dan batas kandidat minimal tiga. Jika hasil awal belum memuat hubungan nama eksplisit, mesin menambahkan query biografi. Query tambahan tetap diperiksa terhadap nama input asli, tetap memakai budget dan batas waktu yang sama, dan tercatat pada `search.research`. Snippet dengan hubungan nama eksplisit diprioritaskan dalam ranking. Username saja, kemiripan nama, atau ingatan model tidak menjadi bukti nama asli.

Profesi majemuk yang jelas dapat dipertahankan sebagai klaim terpisah. Nama panggung terdahulu tidak mengubah profesi aktif menjadi pekerjaan historis. Institusi pendidikan tetap terpisah dari employer; company yang belum didukung bukti pekerjaan tetap kosong.

## Validasi 25-26 Oktober 2026

Pencarian langsung dilakukan pada 17 query, memakai batas bawaan lima kandidat dan 20 request. Semua run memiliki status cakupan `PARTIAL`. Delapan nama tambahan diuji selain tiga contoh awal, kemudian enam nama lain diuji sebelum hasilnya diperiksa. Temuan dari kedua tahap dipakai untuk perbaikan. Karena itu, kumpulan ini sekarang merupakan regression set, bukan pengukuran akurasi independen.

| Query | Klaim nama pada replay akhir | Catatan |
| --- | --- | --- |
| IShowSpeed | Darren Jason Watkins Jr. | Nama dan profesi terbaca tanpa Ollama |
| PewDiePie | Felix Arvid Ulf Kjellberg | Query biografi membantu memperoleh bukti |
| Rich Brian | Brian Imanuel Soewarno | Keterangan alias terdahulu tidak lagi membuat profesi historis |
| Lady Gaga | Stefani Joanne Angelina Germanotta | Klaim dari sumber biografi |
| Bruno Mars | Peter Gene Hernandez; Peter Eugene Hernandez | Ejaan sumber berbeda, tidak dipilih otomatis |
| The Weeknd | Abel Tesfaye | Tidak melengkapi nama dengan pengetahuan di luar sumber |
| Lorde | Ella Marija Lani Yelich-O'Connor | Apostrof dan tanda hubung dipertahankan |
| MrBeast | Jimmy Donaldson | Tidak mengubah nama sumber menjadi versi yang lebih panjang |
| Cak Lontong | Lies Hartono | Mendukung pola nama panggung berbahasa Indonesia |
| Sting | Gordon Matthew Thomas Sumner; Steve Borden | Kandidat musisi dan pegulat tetap terpisah |
| Banksy | Tidak ada klaim nama yang diterima | Tidak membuat tebakan nama dari hasil yang terkumpul |
| Eminem | Marshall Bruce Mathers III | Akhiran nama dipertahankan |
| Snoop Dogg | Calvin Cordozar Broadus Jr. | Titik pada Jr. tidak memutus atribusi |
| Doja Cat | Amala Ratna Zandile Dlamini | Nama dan profesi terbaca |
| Bono | Paul David Hewson; Yassine Bounou | Gelar dipisahkan; dua orang tetap menjadi kandidat berbeda |
| Agnez Mo | Agnes Monica Muljoto | Klaim dari sumber biografi |
| Deddy Corbuzier | Deodatus Andreas Deddy Cahyadi Sundjojo; Deodatus Andreas Deddy Cahyadi Sunjoyo | Perbedaan ejaan sumber dipertahankan |

Tabel mencatat apa yang dinyatakan sumber, bukan sertifikasi kebenaran nama atau kepemilikan akun. Dalam replay akhir, 16 query menghasilkan setidaknya satu klaim nama; satu query tidak. Angka ini bukan precision atau recall. Sumber indeks dan halaman tujuan juga dapat memuat informasi yang sama dan tidak dihitung sebagai konfirmasi independen.

Uji otomatis mencakup nama sintetis, Unicode, inisial, nama tunggal, klaim negatif, rumor, perusahaan/band, fanpage, nama mirip, kutipan palsu, bukti milik kandidat lain, dua orang dengan alias sama, replay, dan alur review. Uji fuzz memeriksa bahwa nama dan kutipan tetap merupakan bagian literal sumber, bukan bahwa klaim sumber benar.

Cuplikan publik dan ekspektasi per kandidat disimpan dalam [fixture regresi](../fixtures/public-name-snapshots-v1.json). Ini adalah cuplikan yang dikurasi dari snapshot, bukan salinan lengkap halaman. Hash snapshot asal dicatat per kasus. Ekspektasi menilai ekstraksi klaim sumber dan diperiksa setelah output tersedia; tidak ada anotasi penilai independen. Jalankan:

```powershell
go test ./...
go vet ./...
go test ./internal/enrich -run TestLivePublicNameSnapshots -v
go test ./internal/personname -run '^$' -fuzz FuzzNamesRetainLiteralEvidence -fuzztime=20s -parallel=2
```

Benchmark ekstraksi lama tetap menghasilkan TP=14, FP=0, FN=2 pada 24 kasus sintetis, dengan abstensi benar 11/11. Dua kasus yang masih terlewat adalah narasi memakai kata ganti dan pendidikan dalam prosa. Perubahan ini tidak menyelesaikan semua bentuk atribusi.

Perbandingan Ollama `qwen2.5:7b` pada empat input identik (Bruno Mars, Cak Lontong, Lorde, Rich Brian) mempertahankan klaim nama yang sama. Model menghasilkan usulan hubungan tambahan yang tetap perlu review; ia tidak mengisi nama dari ingatan. Hasil terukur disimpan dalam [catatan validasi](../benchmarks/results/2026-10-26-public-names.json).

## Batas yang masih ada

Pola nama yang implisit, bahasa yang belum didukung, halaman yang tidak terambil, dan sumber yang bertentangan masih membutuhkan review. Tidak semua profesi atau employer ditemukan. Tanpa nama atau hubungan eksplisit pada sumber yang terkumpul, field boleh tetap kosong. Langkah evaluasi berikutnya adalah anotasi independen pada snapshot baru yang tidak digunakan untuk memperbaiki aturan, disertai ukuran keberhasilan retrieval dan salah atribusi per kandidat.
