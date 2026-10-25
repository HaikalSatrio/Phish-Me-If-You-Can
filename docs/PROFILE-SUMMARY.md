# Ringkasan setelah konfirmasi identitas

`challenge` sekarang langsung menampilkan ringkasan gabungan setelah keputusan disimpan. Untuk membaca ulang atau mengekspor laporan:

```powershell
.\bin\pmiyc.exe profile -input data/hasil-reviewed.json -out data/profile-report.json
```

Laporan hanya memakai kandidat dengan keputusan `CONFIRM`. Kandidat `UNSURE`, `PENDING`, dan `REJECT` tercatat sebagai profil yang dikecualikan dan tidak menyumbangkan klaim kepada ringkasan. Tanpa profil terkonfirmasi, perintah menampilkan `IDENTITY_REVIEW_REQUIRED`. Pengguna tetap harus menentukan apakah profil-profil tersebut milik orang yang dimaksud.

Nama input pencarian disajikan terpisah dari nama yang diamati pada sumber. Field profil terstruktur seperti workplace, role, website, dan location tetap ditampilkan dengan provenance. Pekerjaan, pendidikan, profesi, serta hubungan lain dikelompokkan terpisah. Klaim identik dapat mempunyai beberapa referensi, tetapi jumlah sumber tidak dianggap bukti independen atau kenaikan confidence.

Setiap klaim mempertahankan kandidat asal, subjek yang tertulis, ID hubungan, metode, keputusan review, kutipan literal, sumber, dan periode. Bukti judul profil dan bukti isi ditampilkan terpisah bila atribusi memerlukan keduanya. Hubungan yang ditolak tidak dipakai; ID-nya tetap tercatat. Usulan Ollama yang belum diterima berada di `pending_relations`; setelah `review-relation ... -decision ACCEPT`, usulan dapat bergabung dengan kelompok klaim sumber. Statusnya tetap belum terverifikasi.

Laporan menandai beberapa employer yang disebut dalam headline atau klaim present-tense, serta employer yang muncul dalam klaim historis dan klaim present-tense/headline. Penanda ini meminta pemeriksaan periode. Orang dapat bekerja di beberapa tempat atau kembali ke employer lama; laporan tidak memilih salah satu atau menyatakan sumber pasti bertentangan.

Laporan adalah hasil turunan untuk dibaca, bukan snapshot input untuk `search`, `challenge`, atau `enrich`. Gunakan snapshot reviewed asli untuk melanjutkan proses. Perintah tidak melakukan request jaringan, ekstraksi ulang, perubahan keputusan, atau penghitungan PES.

## Demo fiktif yang dapat dijalankan ulang

Semua nama, institusi, akun, dan observasi pada fixture berikut adalah data demonstrasi sintetis. URL tidak diambil dari jaringan.

```powershell
.\bin\pmiyc.exe enrich -input fixtures/profile-review-demo.json -out data/demo-enriched.json
.\bin\pmiyc.exe challenge -input data/demo-enriched.json -select "1,2" -reason "Two profiles of the same fictional demo subject" -out data/demo-reviewed.json
.\bin\pmiyc.exe profile -input data/demo-reviewed.json -out data/demo-profile.json
```

Ringkasan akan mempertahankan variasi nama `Lila` dan `Lila Utami`, profesi pada Instagram, pekerjaan di Cedar University, serta pendidikan di Harbor University. Fixture ini menguji alur aplikasi, bukan keberhasilan penemuan profil nyata. Output tidak menimpa file yang sudah ada.
