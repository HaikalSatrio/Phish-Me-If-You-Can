# Ekstraksi hubungan dengan bukti

Pencarian sekarang menjalankan `relations-v3`. Ekstraktor company/role lama tetap tersedia sebagai baseline pengujian, tetapi tidak lagi mengisi hasil pencarian. Penyebutan nama di suatu halaman tidak cukup untuk mengaitkan semua organisasi di halaman itu dengan orang tersebut.

Pipeline menyimpan hubungan pekerjaan (`employment`), jabatan publik (`public_office`), profesi (`occupation`), pendidikan (`education`), keanggotaan (`membership`), dan tempat lahir (`birthplace`). Setiap hubungan memiliki subjek, kutipan literal, ID observasi sumber, metode, periode jika tertulis, dan status `UNVERIFIED`. URL, waktu observasi, serta hash respons tersedia melalui observasi yang dirujuk.

Ringkasan memakai hubungan deterministik atau usulan yang sudah diterima operator. `present_assertion` berarti sumber memakai bentuk kini, bukan jaminan jabatan masih berlaku saat ini. `headline` berarti judul/deskripsi profil tanpa periode. Jika hanya tersedia metadata berperiode `unknown`, nilai dapat tampil dengan label tersebut; bukti unknown yang mendukung nilai sama tidak lagi menghapus ringkasan. Field profil terstruktur yang sudah ada tetap dipertahankan.

Beberapa role ditampilkan dengan pemisah titik koma dan penanda `multiple_claims`. Ini daftar klaim, bukan kepastian pekerjaan bersamaan. `role_claims` dan `company_claims` menyimpan jenis, periode, serta referensi hubungan masing-masing, termasuk riwayat. Perusahaan berbeda tetap ambigu pada field company tunggal. Periode historis tidak mengisi field pekerjaan saat ini; klaim historis dan pernyataan kini untuk nilai sama tetap ditandai konflik yang memerlukan pemeriksaan. Ringkasan role memakai pemetaan English canonical; nilai dan kutipan hubungan tetap dalam bahasa sumber. Tidak ada pasangan role/company baru yang dibentuk dari daftar klaim.

Label seperti `Head of Cyber Security Program` dan `Head of Cybersecurity` disimpan sebagai `occupation` dengan role lengkap, bukan perusahaan. Sebaliknya, `CEO of Tesla` menjadi `employment` dengan role `CEO` dan organization `Tesla`; `Treasurer of HIMTI BINUS University` tetap `membership`. Headline yang hanya berisi daftar skill seperti `Cybersecurity & Web Development` tidak dianggap role. Hubungan dengan periode historis tidak berubah menjadi headline dan tidak mengisi ringkasan.

Riwayat tidak menjadi pekerjaan saat ini. Contoh kutipan `Joko Widodo ... Presiden Indonesia ketujuh dari tahun 2014 sampai 2024` menghasilkan jabatan publik historis, bukan perusahaan. Nama rumah sakit dalam berita kelahiran dan jabatan orang lain dalam tabel kabinet tidak menjadi perusahaan/jabatan subjek. Hasil bergantung pada teks yang berhasil dikoleksi, bukan pengetahuan umum yang dimasukkan diam-diam.

`public-page-v3` mempertahankan batas paragraf, baris tabel, dan pemisah sel. JSON-LD `Person` diekstrak sebagai observasi `person_jsonld` yang dibatasi; `name`, `jobTitle`, `worksFor`, `alumniOf`, `memberOf`, dan `birthPlace` dipisahkan per orang. Konteks dan referensi eksternal tidak diambil. Nama harus cocok persis setelah normalisasi spasi/huruf. Periode JSON-LD tetap `unknown`; beberapa employer/job title tidak dipasangkan melalui perkalian silang. Tabel bebas belum mempunyai parser hubungan umum.

## Memproses snapshot lama tanpa pencarian ulang

```powershell
.\bin\pmiyc.exe enrich -input data/hasil-lama.json -out data/hasil-relasi.json
.\bin\pmiyc.exe candidates -input data/hasil-relasi.json
```

Input asli tidak diubah dan output tidak boleh menimpa file. Inferensi lama tetap disimpan dengan `superseded_by: relations-v3` untuk audit, tetapi tidak lagi menjadi ringkasan. Memproses ulang snapshot tidak memulihkan teks yang terpotong atau struktur HTML yang sudah hilang. Jalankan pencarian baru jika memerlukan observasi baru.

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

Setiap kandidat mengirim maksimal satu konteks berbatas 12.000 karakter, dimulai di sekitar nama yang cocok. Default maksimal lima request generasi lokal, masing-masing 90 detik; response maksimal 256 KiB, 12 usulan, dan 3.072 token keluaran. Batas session pencarian tetap berlaku. `enrich` menyediakan `-timeout`, `-max-requests`, dan `-ollama-url`. Setiap generasi juga mencoba membaca metadata lokal `/api/tags` dengan batas tiga detik dalam deadline yang sama; metadata ini tidak dihitung sebagai request generasi.

`extraction` menyimpan hash input/output/prompt, opsi runtime, digest bobot jika tersedia, durasi, serta alasan penolakan per usulan. Status model membedakan `EMPTY`, `REJECTED_ALL`, `PARTIAL`, `SUCCESS`, `FAILED`, dan `SKIPPED`. Kegagalan layanan memiliki `error_code`; penolakan usulan memiliki `rejections[].code`, detail, dan usulan asal. `accepted` berarti lolos validator, bukan diterima operator atau terbukti benar. Koreksi yang mempunyai dukungan deterministik dicatat pada `adjustments`, termasuk pemisahan jenis hubungan, judul jabatan publik, fungsi yang diduplikasi menjadi organisasi, dan kata kerja yang keliru menjadi role. Tidak ada tahun baru yang dikarang untuk melengkapi usulan.

Tambahkan `-debug-extraction` pada `enrich` atau `extraction.save_raw_response: true` pada konfigurasi untuk menyimpan respons mentah lokal yang dibatasi ukurannya. Default tidak menyimpan respons mentah. Informasi kutipan dan usulan yang ditolak tetap tercatat tanpa flag tersebut.

Kutipan harus benar-benar ada pada observasi milik kandidat; subjek, organisasi, jabatan dan tahun harus didukung literal oleh kutipan. Perbedaan whitespace dapat dipulihkan ke kutipan asli dengan catatan koreksi. Respons rusak, terpotong, berlebih, atau memakai ID bukti lain ditolak. JSON Schema membatasi subjek dan ID dokumen ke input aktual. Instruksi dalam halaman diperlakukan sebagai data. Validasi literal belum membuktikan hubungan semantik: kutipan dapat menyebut beberapa orang. Karena itu usulan model tetap terlihat sebagai `ollama/relations-v3`, memerlukan pemeriksaan hubungan, dan tidak otomatis mengisi ringkasan atau PES. Konfirmasi identitas kandidat juga tidak memverifikasi hubungan tersebut.

## Review hubungan

Lihat ID, kutipan, metode, dan status review dengan `candidates`, lalu buat snapshot keputusan:

```powershell
.\bin\pmiyc.exe candidates -input data/hasil-hybrid.json
.\bin\pmiyc.exe review-relation -input data/hasil-hybrid.json -out data/hasil-reviewed.json -relation "ID_DARI_OUTPUT" -decision ACCEPT -reason "Kutipan sumber menyatakan hubungan ini secara eksplisit"
```

`ACCEPT` membuat usulan model layak dipertimbangkan dalam ringkasan sesuai aturan periode dan konflik. `REJECT` mengecualikan hubungan dari ringkasan. `PENDING` mencabut keputusan sebelumnya; usulan model kembali memerlukan review, sedangkan hubungan rules mengikuti aturan default. Riwayat keputusan, waktu, dan alasan tetap disimpan. ID hubungan stabil untuk tuple kandidat, dokumen, subjek, jenis, nilai, dan periode yang sama; keputusan ikut dipertahankan saat `enrich` dijalankan ulang. Sumber input tidak diubah. Review ini tidak mengubah identitas, status `UNVERIFIED`, atau kelayakan PES.

## Batas dan evaluasi

- Aturan prosa terutama mencakup Indonesia dan Inggris. Pengurangan tebakan meningkatkan abstention; beberapa judul profil yang dulu menghasilkan field sekarang hanya tersimpan sebagai observasi.
- Alias/typo seperti `Jokowi dodo` tidak otomatis menjadi `Joko Widodo`. Alias yang tertulis dalam biografi membantu parsing nama kanonik, bukan penggabungan identitas antarhalaman.
- Parser mengenali tahun, tanggal ISO, serta tanggal lengkap Inggris/Indonesia, tetapi periode tersimpan tetap berpresisi tahun. Tahun akhir sampai tahun evaluasi diperlakukan historis secara konservatif; status dalam bulan/hari tertentu belum dimodelkan penuh.
- Sumber dapat salah, usang, atau saling bertentangan. Kandidat dari URL berbeda tidak digabung otomatis, dan konflik lintas kandidat belum direkonsiliasi.
- `go test ./internal/extract -run TestRelationshipChallenge -v` membandingkan kasus atribusi terhadap baseline lama. Ini corpus regresi yang diketahui saat implementasi, bukan pengukuran akurasi pada data independen.
- `TestRelationRegressionEvaluation` mengukur company, role, tuple hubungan, abstention, kesalahan periode/subjek, serta durasi pada corpus regresi yang sama untuk rules dan model lokal opsional. Lihat [VALIDATION.md](VALIDATION.md) untuk perintah dan hasil. Corpus independen tetap diperlukan untuk menilai generalisasi. Implementasi ini tidak menyatakan kesiapan Black Hat Arsenal.
