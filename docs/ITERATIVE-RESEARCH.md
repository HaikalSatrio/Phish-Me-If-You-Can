# Penelusuran publik bertahap

`search -source web` memakai sumber yang ditemukan sebagai petunjuk untuk permintaan berikutnya. Planner ini deterministik dan gratis. Ollama tetap opsional untuk mengekstrak hubungan dari teks; model belum merencanakan pencarian atau mengendalikan browser.

```powershell
powershell -File scripts/start-local-search.ps1
.\bin\pmiyc.exe search -source web -name "Haikal" -account "instagram:@haikal_satrio" -account "github:HaikalSatrio" -out data/research.json
.\bin\pmiyc.exe next -input data/research.json
.\bin\pmiyc.exe enrich -input data/research.json -model qwen2.5:7b -out data/research-hybrid.json
```

## Alur

1. Baca URL atau akun yang diberikan. Simpan metadata dan teks beserta hash respons.
2. Ikuti tautan bio, portfolio, `rel=me`, halaman about pada situs yang sama, atau repositori README profil yang benar-benar tertaut. Halaman bio diprioritaskan di atas tambahan akun sosial; navigasi About pada situs yang sama ikut diperiksa. Repositori fork dan tautan acak tidak menjadi petunjuk pendidikan.
3. Jalankan pencarian akun pada platform asal. Sisihkan sebagian request dan satu slot hasil untuk penelusuran lanjutan bila limit memungkinkan.
4. Cari display name yang benar-benar diamati, lalu handle di seluruh web. Bila input hanya nama depan, pemisahan handle seperti `LailaKartika` atau `laila_kartika` dapat menghasilkan hipotesis `Laila Kartika`.
5. Baca halaman hasil dan gunakan nama atau tautan baru untuk langkah berikutnya. Nama hipotesis tidak pernah disalin menjadi field nama yang diamati.
6. Ekstrak hubungan per halaman. Tampilkan pendidikan sebagai `education_claims`, terpisah dari `company_claims`. Semua klaim tetap memerlukan pemeriksaan identitas dan periode.

Nama halaman diambil dari field display name GitHub, judul profil Instagram yang cocok dengan handle, atau satu objek Person yang namanya cocok dengan heading utama. Sumber lain dapat tetap mempunyai nama kosong. Nama yang diamati menjadi subjek ekstraksi untuk kandidat itu, sehingga input nama depan tidak menggantikan nama lengkap dalam sumber.

Judul profil LinkedIn juga dapat menyediakan nama teramati. Nama profil yang merupakan subset berurutan minimal dua kata dari input dapat dipertahankan untuk review, tanpa konfirmasi otomatis. Pencarian SearXNG menyediakan variasi nama penuh dan pendek khusus LinkedIn; akun yang diberikan tetap disertai query nama lintas platform. Cuplikan berbeda dari beberapa query untuk URL yang sama disimpan terpisah beserta provenance. Lihat [batas pencocokan nama](IDENTITY-REVIEW.md).

## Jejak dan batas

`search.research` menyimpan tindakan, query, URL asal/tujuan, alasan, status, dan jumlah request. `discovery_basis` dan `linked_from` menjelaskan asal kandidat. Tautan balik eksplisit menuju akun yang diberikan dicatat dalam `matches` dengan referensi evidence. Tautan ini tidak menaikkan confidence atau mengonfirmasi kepemilikan akun.

Penelusuran query lanjutan aktif jika ada petunjuk akun/URL dan budget awal setidaknya delapan request. Maksimum tiga query lanjutan dan empat pengambilan tautan bio per koleksi. Semua permintaan, termasuk redirect, memakai budget dan deadline sesi yang sama. Kandidat tetap dibatasi `candidate_limit`; saat limit penuh, penelusuran berhenti. Nama dan hubungan dari URL berbeda belum digabung menjadi satu identitas. Layanan yang gagal karena jaringan atau challenge tidak dicoba lagi untuk setiap petunjuk dalam koleksi yang sama; kegagalan menemukan hasil untuk satu query tidak menonaktifkan layanan.

`next` mendahulukan permintaan URL bio publik jika penelusuran belum menghasilkan nama, pendidikan, perusahaan, atau role. Pertanyaan identitas menyertakan kartu kandidat dan kutipan sumber. `challenge` atau `search -interactive` menerima pilihan pengguna dan menyimpan asosiasi profil secara eksplisit; pertanyaan tidak membuat keputusan otomatis. Lihat [alur konfirmasi identitas](IDENTITY-REVIEW.md).

## Validasi

Fixture menguji rantai akun → website → bio, sumber nama lengkap, pendidikan terpisah, backlink, handle sebagai hipotesis, siklus URL, budget, pembatalan, sumber tidak tersedia, dan pencegahan transfer klaim antarorang. Fixture bukan ukuran akurasi web secara umum.

Uji langsung 25 Oktober 2026 memakai input `Haikal` dan dua handle di atas, tanpa memasukkan nama lengkap dari percakapan. SearXNG sempat tidak aktif dan dijalankan kembali. Setelah layanan siap, satu pengambilan menghasilkan lima halaman dalam 16/20 request: dua akun awal, dua repositori, dan satu profil bernama mirip. Tidak ada nama lengkap, kampus, atau pekerjaan yang berhasil diatribusikan kepada pemilik kedua akun. Hasil ini menunjukkan penelusuran tambahan berjalan, tetapi belum membuktikan keberhasilan identifikasi. Indeks dan ketersediaan sumber berubah antarpercobaan.

Keterbatasan berikutnya adalah pemisahan kuota dokumen dari kuota kandidat, ekstraksi nama untuk lebih banyak format bio, dan evaluasi resolusi identitas lintas sumber pada corpus independen. Implementasi ini belum menyatakan kesiapan Black Hat Arsenal.

Uji pembanding dengan input `Julia` dan `github:jvns` mendapatkan display name `Julia Evans` dari sumber GitHub dan mengikuti tautan situs publiknya. Tidak ada company, role, atau pendidikan yang berhasil diekstrak dalam snapshot tersebut. Uji ini juga mengungkap perlunya mendahulukan halaman About dibanding tambahan profil sosial; prioritas itu ditambahkan dan dicakup tes regresi.
