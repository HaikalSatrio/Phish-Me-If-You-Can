# Perbandingan rules dan Ollama pada nama baru

Tanggal pengujian: 24 Oktober 2026. Nama Katie Moussouris, Meredith Whittaker, dan Enda Nasution tidak ditemukan dalam pencarian teks snapshot lokal sebelum pengujian ini. Nama dipilih untuk mencakup biografi Inggris, jabatan di organisasi nirlaba, dan profil Indonesia. Sampel ini kecil dan dipakai untuk diagnosis serta pengembangan, bukan benchmark independen atau pengukuran akurasi umum.

## Metode

Setiap nama dicari sekali menggunakan `search -source web`, konfigurasi gratis yang sudah ada, dan batas lima kandidat. Ketiganya menghasilkan status PARTIAL dengan masing-masing lima kandidat; jumlah observasi adalah 25, 26, dan 27. Snapshot koleksi yang sama kemudian diproses oleh rules dan Qwen pada versi sebelum perubahan (`0a1375d`, `relations-v3`) dan versi setelah perubahan (`relations-v4`). Tidak ada pencarian ulang untuk memperbaiki hasil perbandingan. Hash observasi OBSERVED cocok pada semua replay.

Model yang digunakan adalah `qwen2.5:7b` lokal, digest `845dbda0ea48ed749caafd9e6037047aa19acfcfd82e704d7ca97d631a0b697e`, temperature 0, num_ctx 8192, num_predict 3072, dan batas lima generasi per snapshot. Raw response dan pemetaan potongan sumber disimpan untuk pemeriksaan.

## Field yang tampil tanpa Ollama

Angka berikut adalah jumlah kandidat dengan field terisi dari lima kandidat per nama. Ini ukuran keterisian, bukan precision. Semua nilai tetap merupakan klaim sumber yang belum diverifikasi.

| Nama | Company sebelum | Company sesudah | Role sebelum | Role sesudah |
| --- | ---: | ---: | ---: | ---: |
| Katie Moussouris | 1, salah | 4 | 4 | 5 |
| Meredith Whittaker | 0 | 2 | 1 | 2 |
| Enda Nasution | 0 | 0 | 0 | 2 |

Kesalahan company `vulnerability disclosure` dihapus. Bentuk founder/CEO kini dipisahkan dari Luta Security, termasuk pada [halaman resmi perusahaan](https://www.lutasecurity.com/founder-ceo). Role Chief Policy Officer dipisahkan dari HackerOne pada snippet [Black Hat USA 2015](https://blackhat.com/us-15/speakers/Katie-Moussouris.html). Halaman tersebut adalah sumber lama; pernyataan berbentuk kini di dalamnya tidak membuktikan pekerjaan saat ini.

Snapshot Meredith kini menghasilkan president/Signal Foundation dari pengantar biografi dan President/Signal dari bentuk posesif. [Pengumuman Burda](https://www.burda.com/en/news/meredith-whittaker-joins-burdas-board-directors/) juga menyebut perannya di Signal. Pada Enda, profesi penulis pada pengantar biografi serta Digital Activist, Co-Founder, dan Strategic Communication Advisor & Startup Mentor pada headline tampil terpisah. Company tetap kosong karena daftar organisasi pada headline belum direkonsiliasi menjadi perusahaan individual.

Salah satu hasil pencarian adalah Sorie Enda Nasution. Pengantar biografinya menyebut nama yang lebih panjang dan tidak boleh digunakan sebagai hubungan Enda Nasution. Model sempat mengusulkan jabatan dari halaman tersebut; validator v4 menolaknya dengan `SUBJECT_NAME_CONFLICT`, terlepas dari validitas tahun yang diajukan.

## Kontribusi Ollama

| Nama | Usulan lolos sebelum | Ditolak sebelum | Usulan lolos sesudah | Ditolak sesudah |
| --- | ---: | ---: | ---: | ---: |
| Katie Moussouris | 1 | 15 | 5 | 1 |
| Meredith Whittaker | 2 | 11 | 4 | 3 |
| Enda Nasution | 0 | 3 | 4 | 2 |
| Total | 3 | 29 | 13 | 6 |

Lolos berarti lulus pemeriksaan sumber dan struktur. Angka ini bukan jumlah fakta benar atau fakta baru: sebagian usulan mengulang hasil rules, hanya berbeda kapitalisasi, atau mengambil satu bagian dari jabatan gabungan. Setelah deduplikasi pipeline, terdapat enam baris model tambahan dibanding replay rules. Contohnya adalah klaim President of Signal pada snippet profil pembicara dan label Tokoh Blogger Indonesia pada transkrip kedutaan. Model juga mempertahankan `Suvarna.ID & HiPajak` sebagai satu nilai organisasi; operator harus memeriksa interpretasi daftar tersebut sebelum menerima hubungan.

Jumlah field ringkasan pada mode hybrid sama dengan rules sampai hubungan model diterima melalui `review-relation`. Konfirmasi identitas kandidat tidak menerima hubungan model. Penerimaan operator juga tidak membuat klaim sumber otomatis benar atau memenuhi syarat PES.

Total durasi generasi yang tercatat pada 15 kandidat turun dari 135.929 detik ke 32.967 detik dalam satu pass ini. Input model dan cara menghasilkan kutipan berubah, sehingga angka tersebut bukan jaminan peningkatan kecepatan pada beban lain. Tidak dilakukan pengulangan untuk mengukur variasi runtime atau mengendalikan kondisi pemanasan model.

## Perubahan yang diuji

- Rules menangani preposisi `for`, jabatan `of` organisasi, bentuk posesif, headline dengan koma, pola organisasi-jabatan-nama, dan pengantar biografi dengan tanggal lahir.
- Frasa bidang setelah `in` tidak langsung menjadi employer; institusi dengan penanda jelas tetap didukung.
- Potongan input mempertahankan teks sumber, menghapus duplikasi, membatasi konteks, dan mencapai penyebutan nama yang lebih jauh dalam observasi panjang.
- Model memilih ID potongan pendek; kutipan dipasang aplikasi dari sumber asli, bukan ditulis ulang model.
- Validator menolak nama personal yang lebih panjang pada pengantar biografi dan badan kolektif yang dijadikan jabatan individu. Koreksi periode serta jenis hubungan tetap memerlukan dukungan sumber.

`go test ./...`, `go vet ./...`, dan build Windows lulus. Corpus regresi 13 kasus sebelumnya juga dijalankan pada Qwen nyata: sepuluh hubungan yang diharapkan dipertahankan, empat kasus negatif tidak menghasilkan hubungan yang dipertahankan. Ini corpus yang sudah diketahui, bukan bukti generalisasi.

## Artefak dan reproduksi

Artefak lokal berada pada ignored `data/new-name-comparison-20261024/`:

- `*-before-rules.json`: hasil koleksi awal.
- `*-before-hybrid.json`: Qwen sebelum perubahan.
- `*-final-rules.json` dan `*-final-hybrid.json`: replay setelah perubahan.
- `comparison.json`: keterisian field, jumlah usulan, durasi, dan hash observasi/input.
- `regression-v4.json`: keluaran corpus regresi dengan model nyata.

Contoh replay yang memakai snapshot identik:

```powershell
.\bin\pmiyc.exe enrich -input data/new-name-comparison-20261024/katie-before-rules.json -out data/katie-rules-replay.json
.\bin\pmiyc.exe enrich -input data/new-name-comparison-20261024/katie-before-rules.json -out data/katie-hybrid-replay.json -model qwen2.5:7b -debug-extraction
```

Pengujian ini menunjukkan masalah rules yang dapat diperbaiki dan peningkatan usulan model yang dapat diperiksa. Masih diperlukan penilaian independen atas atribusi orang, periode yang sebenarnya berlaku, konflik antarsumber, daftar beberapa organisasi, dan profil dengan bukti tipis. Tidak ada klaim kesiapan Black Hat Arsenal dari sampel ini.
