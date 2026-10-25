# Konfirmasi profil dan klaim sumber

`challenge` menampilkan nama, URL, klaim pekerjaan atau pendidikan, kutipan, serta sumber setiap kandidat. Pertanyaannya adalah profil mana yang dimaksud pengguna. Memilih profil tidak memverifikasi pekerjaan atau periode dalam sumber.

```powershell
.\bin\pmiyc.exe enrich -input data/hasil.json -out data/hasil-enriched.json
.\bin\pmiyc.exe challenge -input data/hasil-enriched.json -out data/hasil-reviewed.json
```

- Masukkan nomor profil dipisahkan koma untuk mengaitkan satu atau beberapa profil dengan target. Profil lain menjadi `UNSURE`.
- `none` menolak seluruh kandidat; `unsure` menyatakan seluruh kandidat belum pasti.
- Masukkan alasan keputusan. `q` pada pemilihan atau input terputus membatalkan tanpa menyimpan hasil parsial.
- Kandidat dengan petunjuk bertentangan tidak dapat dikonfirmasi. Perbaiki petunjuk dan lakukan pencarian baru.

Untuk otomasi, gunakan `-select "1,2" -reason "Alasan berdasarkan pemeriksaan operator"` setelah memeriksa kartu pada snapshot yang sama. Nomor mengikuti urutan kandidat dalam snapshot. Gunakan ID kandidat pada perintah `review` untuk mengubah satu keputusan; `review ... -decision CONFIRM` tetap memilih satu kandidat utama dan mengembalikan konfirmasi kandidat lain ke `PENDING`.

Pencarian baru dapat langsung menanyakan identitas:

```powershell
.\bin\pmiyc.exe search -source web -name "Nama yang dicari" -account "github:handle" -interactive -out data/pencarian.json
```

Snapshot koleksi disimpan dahulu. Keputusan disimpan ke `data/pencarian-reviewed.json`. Output tidak boleh menimpa file. `next -input ...` hanya menampilkan pertanyaan tanpa meminta jawaban.

## Ekstraksi tanpa Ollama

Nama satu kata pada judul profil Instagram diterima jika handle cocok dengan URL profil. Judul LinkedIn dapat menyediakan nama teramati. Postingan dan reel tidak diperlakukan sebagai profil pemilik bio.

`profile-binding-v1` mengaitkan judul profil dengan potongan bio atau headline yang memenuhi pola terbatas. `subject_evidence_id` dan `subject_quote` menyimpan bukti nama; `evidence_id` dan `quote` menyimpan bukti hubungan. Validator memeriksa keduanya pada kandidat yang sama. Kutipan tidak digabung menjadi kalimat buatan. Headline terpotong tidak menghasilkan jabatan yang menggantung. Company tetap kosong jika hanya profesi yang didukung.

Replay snapshot Nadia pada 25 Oktober 2026, tanpa Ollama, menghasilkan:

| Sumber | Nama teramati | Role | Company |
| --- | --- | --- | --- |
| Profil Instagram | Nadia | Cyber Security Lecturer | Kosong |
| Hasil profil LinkedIn | Nadia Nadia | Deputy Head of Cyber Security Program | Bina Nusantara University |

Kedua hubungan berstatus `UNVERIFIED`, berperiode `headline`, dan kedua identitas tetap `PENDING`. Ini hasil pengolahan snapshot tersimpan, bukan verifikasi pekerjaan saat ini atau pengambilan web baru. Sumber postingan yang menyebut kampus lain tidak dipindahkan menjadi employer. Pengujian ini merupakan regresi kasus yang sudah diketahui, bukan benchmark akurasi independen.

## Penyimpanan dan batas

`identity_reviews` menyimpan waktu, alasan, dan keputusan lengkap untuk seluruh kandidat. Beberapa profil hanya dapat dikonfirmasi dalam sesi pencarian dengan catatan eksplisit yang konsisten. Sumber asli, kutipan, dan keputusan hubungan tetap dipertahankan. `enrich` mempertahankan riwayat; koleksi baru membentuk kandidat baru yang perlu direview kembali.

Company dan role tetap merupakan ringkasan per profil. Asosiasi identitas belum menggabungkan seluruh klaim menjadi satu biografi. Nama profil, tautan profil, dan hubungan yang belum terverifikasi tidak masuk PES hanya karena identitas dikonfirmasi. Usulan Ollama tetap memerlukan `review-relation` untuk dipakai dalam ringkasan.

Pola bio belum mencakup semua bahasa, format, atau cara menyebut pekerjaan. Sumber tertutup, halaman gagal diambil, dan teks yang tidak lengkap tetap membatasi hasil. Tahap berikutnya adalah corpus evaluasi independen untuk mengukur salah atribusi, kelengkapan ekstraksi, dan ketepatan abstensi sebelum memperluas integrasi atau antarmuka graph.

## Nama profil yang dipendekkan

Pencarian dapat mempertahankan profil Instagram atau LinkedIn yang judulnya memuat sedikitnya dua kata nama dalam urutan yang sama dengan input, tanpa kata nama tambahan. Contoh: input `Raka Aditya Pranata` dapat menemukan `Raka Aditya`, tetapi aturan sebagian tidak menerima `Muhammad Raka Aditya Wibowo`. Handle eksplisit tetap harus cocok. Kandidat ditandai sebagai kecocokan nama sebagian, tetap `PENDING`, dan tidak memperoleh konfirmasi atau kenaikan confidence dari aturan ini.

Untuk nama tiga kata atau lebih, SearXNG mencoba query penuh, query penuh khusus LinkedIn bila platform belum ditentukan, dan dua kata pertama khusus LinkedIn. Variasi tetap dibatasi budget. Pemberian akun tidak menghapus pencarian nama lintas platform. Direktori LinkedIn dan daftar nama Facebook tidak diperlakukan sebagai profil individu.

Cuplikan berbeda untuk URL yang sama dipertahankan dengan ID bukti dan sumber query masing-masing, dibatasi delapan nilai per tipe observasi. Nilai identik dideduplikasi. Label `Pendidikan:` atau `Education:` pada cuplikan profil yang memenuhi pola dapat menghasilkan hubungan pendidikan dengan kutipan terpisah. Label pengalaman tidak otomatis dianggap employer.

Uji langsung 25 Oktober 2026 dengan input lengkap `Haikal Satrio Dewandaru` menghasilkan satu kandidat LinkedIn bernama `Haikal Satrio`, role `Undergraduate Cyber Security Student`, dan pendidikan `BINUS University`, tanpa Ollama. Company tetap kosong. Enam dari dua puluh request terpakai. Pengambilan halaman LinkedIn gagal dengan HTTP 999, sehingga bukti berasal dari cuplikan indeks dan belum membuktikan identitas maupun status pendidikan saat ini. Ini regresi kasus yang sudah diketahui, bukan ukuran akurasi umum.
