# Ekstraksi hubungan dengan bukti

Pencarian sekarang menjalankan `relations-v2`. Ekstraktor company/role lama tetap tersedia sebagai baseline pengujian, tetapi tidak lagi mengisi hasil pencarian. Penyebutan nama di suatu halaman tidak cukup untuk mengaitkan semua organisasi di halaman itu dengan orang tersebut.

Pipeline menyimpan hubungan pekerjaan (`employment`), jabatan publik (`public_office`), profesi (`occupation`), pendidikan (`education`), keanggotaan (`membership`), dan tempat lahir (`birthplace`). Setiap hubungan memiliki subjek, kutipan literal, ID observasi sumber, metode, periode jika tertulis, dan status `UNVERIFIED`. URL, waktu observasi, serta hash respons tersedia melalui observasi yang dirujuk.

Ringkasan company/role hanya diisi dari satu hubungan deterministik yang tidak ambigu dan dinyatakan dalam bentuk kini oleh sumber. `present_assertion` berarti sumber memakai bentuk kini, bukan jaminan jabatan masih berlaku saat ini. Field profil terstruktur yang sudah ada tetap dipertahankan. Beberapa hubungan tidak dirangkai menjadi pasangan jabatan/perusahaan baru. Perusahaan berbeda dapat menandakan pekerjaan bersamaan atau konflik; semuanya tetap terlihat dan ringkasan inferensi dikosongkan. Ringkasan role memakai pemetaan English canonical; nilai `role` dan kutipan pada hubungan tetap dalam bahasa sumber untuk audit.

Riwayat tidak menjadi pekerjaan saat ini. Contoh kutipan `Joko Widodo ... Presiden Indonesia ketujuh dari tahun 2014 sampai 2024` menghasilkan jabatan publik historis, bukan perusahaan. Nama rumah sakit dalam berita kelahiran dan jabatan orang lain dalam tabel kabinet tidak menjadi perusahaan/jabatan subjek. Hasil bergantung pada teks yang berhasil dikoleksi, bukan pengetahuan umum yang dimasukkan diam-diam.

`public-page-v3` mempertahankan batas paragraf, baris tabel, dan pemisah sel. JSON-LD `Person` diekstrak sebagai observasi `person_jsonld` yang dibatasi; `name`, `jobTitle`, `worksFor`, `alumniOf`, `memberOf`, dan `birthPlace` dipisahkan per orang. Konteks dan referensi eksternal tidak diambil. Nama harus cocok persis setelah normalisasi spasi/huruf. Periode JSON-LD tetap `unknown`; beberapa employer/job title tidak dipasangkan melalui perkalian silang. Tabel bebas belum mempunyai parser hubungan umum.

## Memproses snapshot lama tanpa pencarian ulang

```powershell
.\bin\pmiyc.exe enrich -input data/hasil-lama.json -out data/hasil-relasi.json
.\bin\pmiyc.exe candidates -input data/hasil-relasi.json
```

Input asli tidak diubah dan output tidak boleh menimpa file. Inferensi lama tetap disimpan dengan `superseded_by: relations-v2` untuk audit, tetapi tidak lagi menjadi ringkasan. Memproses ulang snapshot tidak memulihkan teks yang terpotong atau struktur HTML yang sudah hilang. Jalankan pencarian baru jika memerlukan observasi baru.

## LLM lokal opsional

Mode default tidak memerlukan LLM, Docker, API key, atau pembayaran. Mode hybrid memanggil Ollama lokal melalui `/api/chat` dengan JSON Schema, `stream: false`, dan temperature 0, mengikuti [dokumentasi structured outputs Ollama](https://docs.ollama.com/capabilities/structured-outputs).

Pasang [Ollama](https://ollama.com/download) dan model lokal pilihan Anda. Contoh konfigurasi menggunakan [Qwen2.5 7B](https://ollama.com/library/qwen2.5:7b); ini contoh kompatibilitas, bukan model yang sudah dibuktikan terbaik untuk corpus proyek. Pengunduhan model menggunakan jaringan dan ruang disk, sedangkan inferensi berikutnya berjalan lokal.

```powershell
ollama pull qwen2.5:7b
# Jalankan jika layanan Ollama belum aktif:
ollama serve
```

Pada terminal lain setelah layanan siap:

```powershell
.\bin\pmiyc.exe enrich -input data/hasil-lama.json -out data/hasil-hybrid.json -model qwen2.5:7b
# Atau koleksi baru sekaligus ekstraksi:
.\bin\pmiyc.exe search -source web -name "Joko Widodo" -config configs/collector-local-llm.json
```

Ganti `-model` atau `extraction.model` dengan model yang sudah terpasang. Tidak ada model yang diunduh otomatis. Endpoint hanya menerima origin HTTP loopback; proxy dan redirect dinonaktifkan. SearXNG dan Ollama merupakan layanan terpisah. Kegagalan Ollama menghasilkan diagnostik FAILED untuk tahap model dan mempertahankan hasil rules; keberhasilan scraping tidak berarti tahap model berhasil.

Setiap kandidat mengirim maksimal satu konteks berbatas 12.000 karakter, dimulai di sekitar nama yang cocok. Default maksimal lima request lokal, masing-masing 90 detik; response maksimal 256 KiB, 12 usulan, dan 3.072 token keluaran. Batas session pencarian tetap berlaku. `enrich` menyediakan `-timeout`, `-max-requests`, dan `-ollama-url`. Hash input/output, versi metode/prompt, jumlah usulan diterima/ditolak, durasi, model yang diminta, dan kegagalan disimpan pada `extraction`. Versi metode mengikuti `relations-v2`; tag model saja bukan fingerprint bobot model.

Kutipan harus benar-benar ada pada observasi milik kandidat; subjek, organisasi, jabatan dan tahun harus didukung literal oleh kutipan. Respons rusak, terpotong, berlebih, atau memakai ID bukti lain ditolak. Instruksi dalam halaman diperlakukan sebagai data. Validasi literal belum membuktikan hubungan semantik: kutipan dapat menyebut beberapa orang. Karena itu usulan model tetap terlihat sebagai `ollama/relations-v2`, memerlukan pemeriksaan hubungan, dan tidak otomatis mengisi ringkasan atau PES. Konfirmasi identitas kandidat juga tidak memverifikasi hubungan tersebut.

## Batas dan evaluasi

- Aturan prosa terutama mencakup Indonesia dan Inggris. Pengurangan tebakan meningkatkan abstention; beberapa judul profil yang dulu menghasilkan field sekarang hanya tersimpan sebagai observasi.
- Alias/typo seperti `Jokowi dodo` tidak otomatis menjadi `Joko Widodo`. Alias yang tertulis dalam biografi membantu parsing nama kanonik, bukan penggabungan identitas antarhalaman.
- Periode menggunakan presisi tahun. Tahun akhir sampai tahun evaluasi diperlakukan historis secara konservatif; bulan/hari dan tanggal berakhir di masa depan belum dimodelkan penuh.
- Sumber dapat salah, usang, atau saling bertentangan. Kandidat dari URL berbeda tidak digabung otomatis, dan konflik lintas kandidat belum direkonsiliasi.
- `go test ./internal/extract -run TestRelationshipChallenge -v` membandingkan kasus atribusi terhadap baseline lama. Ini corpus regresi yang diketahui saat implementasi, bukan pengukuran akurasi pada data independen.
- Tes endpoint lokal memakai respons fixture. Akurasi dan latensi model nyata perlu diukur terpisah dengan model, versi bobot, hardware, corpus, precision/recall per field, kesalahan subjek, kesalahan periode, dan abstention yang dilaporkan. Implementasi ini tidak menyatakan kesiapan Black Hat Arsenal.
