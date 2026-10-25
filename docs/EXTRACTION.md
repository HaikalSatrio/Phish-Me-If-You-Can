# Ekstraksi hubungan dengan bukti

Pencarian sekarang menjalankan `relations-v8`. Ekstraktor company/role lama tetap tersedia sebagai baseline pengujian, tetapi tidak lagi mengisi hasil pencarian. Penyebutan nama di suatu halaman tidak cukup untuk mengaitkan semua organisasi di halaman itu dengan orang tersebut.

Versi v8 menolak usulan model yang mengaitkan profesi eksplisit kerabat atau teman kepada subjek, serta usulan pendidikan yang memakai judul role sebagai institusi. Perubahan berasal dari [benchmark pada input yang sama](BENCHMARK-BASELINE.md); aturan tersebut belum mencakup semua bentuk atribusi bahasa alami.

Pola `profile-binding-v1` mendukung bukti terpisah dari judul dan bio/headline profil Instagram atau LinkedIn. Nama satu kata diterima pada format profil yang cocok. Nama disimpan dalam `subject_quote` dengan `subject_evidence_id`; hubungan tetap memakai kutipan literal dan `evidence_id` sendiri. Keduanya harus berasal dari kandidat yang sama dan lolos validasi pola profil. Ini tidak melonggarkan grounding usulan Ollama. Headline yang terpotong tidak dipakai untuk mengisi jabatan yang tidak lengkap. Lihat [konfirmasi profil dan batas ekstraksi](IDENTITY-REVIEW.md).

Field company tetap aktif. Snapshot baru menyimpan `company_status`, dan CLI menjelaskan company kosong: `no_employment_claim` berarti belum ada hubungan pekerjaan yang memenuhi syarat, `pending_review` berarti ada usulan pekerjaan model yang belum diputuskan operator, `historical_only` berarti hanya riwayat, `non_current_claims` berarti hanya klaim dengan periode lain, `ambiguous` berarti beberapa organisasi, dan `period_conflict` berarti periode bertentangan. Company terisi memakai status `available`; status ini tidak memverifikasi klaim. Nama domain atau organisasi yang sekadar muncul di halaman tidak otomatis menjadi company. Jalankan `enrich` untuk menghitung status pada snapshot lama.

Versi 6 membatasi kata seperti `Executive` dan `Chief` agar nama penerbit tidak menjadi jabatan, serta memisahkan uraian tugas seperti `working on ...` dari nama employer. Bio orang pertama didukung secara terbatas: judul halaman harus sama dengan nama lengkap, teks harus dibuka dengan nama itu dan pernyataan `I'm`/`I am`, lalu memiliki klausa employer eksplisit. Narasi wawancara, fiksi, dan subjek lain ditolak. Kutipan tetap berasal dari teks yang sama dan periode dibaca dari klausa pekerjaan yang bersangkutan. Konteks pendek yang mendukung hubungan ini diprioritaskan untuk Ollama, tetapi model tetap dapat salah memilih kutipan atau mengarang tanggal. `present_assertion` berarti sumber menggunakan bentuk kini, bukan verifikasi pekerjaan saat ini. Lihat [validasi versi 6 dan akun lintas platform](COMPARISON-ACCOUNT-QUALITY.md).

Pipeline menyimpan hubungan pekerjaan (`employment`), jabatan publik (`public_office`), profesi (`occupation`), pendidikan (`education`), keanggotaan (`membership`), dan tempat lahir (`birthplace`). Setiap hubungan memiliki subjek, kutipan literal, ID observasi sumber, metode, periode jika tertulis, dan status `UNVERIFIED`. URL, waktu observasi, serta hash respons tersedia melalui observasi yang dirujuk.

Ringkasan memakai hubungan deterministik atau usulan yang sudah diterima operator. `present_assertion` berarti sumber memakai bentuk kini, bukan jaminan jabatan masih berlaku saat ini. `headline` berarti judul/deskripsi profil tanpa periode. Jika hanya tersedia metadata berperiode `unknown`, nilai dapat tampil dengan label tersebut; bukti unknown yang mendukung nilai sama tidak lagi menghapus ringkasan. Field profil terstruktur yang sudah ada tetap dipertahankan.

Beberapa role ditampilkan dengan pemisah titik koma dan penanda `multiple_claims`. Ini daftar klaim, bukan kepastian pekerjaan bersamaan. `role_claims` dan `company_claims` menyimpan jenis, periode, serta referensi hubungan masing-masing, termasuk riwayat. Perusahaan berbeda tetap ambigu pada field company tunggal. Periode historis tidak mengisi field pekerjaan saat ini; klaim historis dan pernyataan kini untuk nilai sama tetap ditandai konflik yang memerlukan pemeriksaan. Ringkasan role memakai pemetaan English canonical; nilai dan kutipan hubungan tetap dalam bahasa sumber. Tidak ada pasangan role/company baru yang dibentuk dari daftar klaim.

Label seperti `Head of Cyber Security Program` dan `Head of Cybersecurity` disimpan sebagai `occupation` dengan role lengkap, bukan perusahaan. Sebaliknya, `CEO of Tesla` menjadi `employment` dengan role `CEO` dan organization `Tesla`; `Treasurer of HIMTI BINUS University` tetap `membership`. Headline yang hanya berisi daftar skill seperti `Cybersecurity & Web Development` tidak dianggap role. Hubungan dengan periode historis tidak berubah menjadi headline dan tidak mengisi ringkasan.

Rules juga membaca bentuk `Chief Policy Officer for Example Labs`, `Example's President`, `Founder & CEO, Example Labs`, dan `Example Labs founder and CEO Nama Lengkap`. Kata bidang seperti `vulnerability disclosure` setelah `in` tidak menjadi perusahaan. Bentuk `in` hanya dipakai sebagai pemisah organisasi jika memiliki penanda organisasi yang jelas. Nama perusahaan dengan kata Security tidak otomatis dianggap fungsi bila didahului jabatan seperti CEO atau founder. Pengantar biografi dengan tanggal lahir dalam kurung dapat menghasilkan profesi, tanpa menjadikan tahun lahir sebagai tahun pekerjaan. Semua pola berlaku umum, tanpa daftar khusus nama orang.

Riwayat tidak menjadi pekerjaan saat ini. Contoh kutipan `Joko Widodo ... Presiden Indonesia ketujuh dari tahun 2014 sampai 2024` menghasilkan jabatan publik historis, bukan perusahaan. Nama rumah sakit dalam berita kelahiran dan jabatan orang lain dalam tabel kabinet tidak menjadi perusahaan/jabatan subjek. Hasil bergantung pada teks yang berhasil dikoleksi, bukan pengetahuan umum yang dimasukkan diam-diam.

`public-page-v4` mempertahankan batas paragraf, baris tabel, pemisah sel, dan header profil atau artikel. Banner situs, navigasi, serta elemen tersembunyi tetap dikecualikan. JSON-LD `Person` diekstrak sebagai observasi `person_jsonld` yang dibatasi; `name`, `jobTitle`, `worksFor`, `alumniOf`, `memberOf`, dan `birthPlace` dipisahkan per orang. Referensi `@id` diselesaikan hanya dari objek pada halaman yang sama, termasuk script JSON-LD terpisah. ID ambigu tidak dipakai, hasil duplikat dibuang, dan penelusuran dibatasi. Konteks dan referensi eksternal tidak diambil. Nama harus cocok persis setelah normalisasi spasi/huruf. Periode JSON-LD tetap `unknown`; beberapa employer/job title tidak dipasangkan melalui perkalian silang. Tabel bebas belum mempunyai parser hubungan umum.

Rules membaca blok profil yang dimulai dengan nama lengkap, termasuk gelar, kemudian baris jabatan eksplisit atau field berlabel `Role`/`Company` dan `Jabatan`/`Perusahaan`. Pembacaan berhenti pada baris yang tidak dikenal. Nama organisasi tanpa label tidak otomatis digabung dengan jabatan. Kutipan tetap satu rentang asli dari satu observasi. Seleksi konteks Ollama mempertahankan field profil bersebelahan pada halaman panjang. Nama merek satu kata dengan huruf kecil dapat diterima pada klausa employer yang eksplisit; kalimat penjelas setelah koma tidak lagi menjadi bagian dari jabatan atau menggantikan employer yang sudah disebut. Lihat [hasil validasi profil](COMPARISON-PROFILE-QUALITY.md).

## Memproses snapshot lama tanpa pencarian ulang

```powershell
.\bin\pmiyc.exe enrich -input data/hasil-lama.json -out data/hasil-relasi.json
.\bin\pmiyc.exe candidates -input data/hasil-relasi.json
```

Input asli tidak diubah dan output tidak boleh menimpa file. Inferensi lama tetap disimpan dengan `superseded_by: relations-v8` untuk audit, tetapi tidak lagi menjadi ringkasan. Memproses ulang snapshot tidak memulihkan teks yang terpotong atau struktur HTML yang sudah hilang. Jalankan pencarian baru jika memerlukan observasi baru.

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

Setiap kandidat mengirim maksimal satu konteks berbatas 12.000 karakter berisi paling banyak 24 potongan sumber, masing-masing maksimal 1.000 karakter. Potongan mempertahankan teks sumber persis, menghapus duplikasi, dan mengambil kemunculan nama yang lebih jauh dalam observasi panjang. Deskripsi halaman dan snippet diprioritaskan sebelum teks panjang. Satu kalimat lanjutan dengan kata ganti dapat ikut disertakan, tetapi atribusi semantiknya masih perlu diperiksa. Default maksimal lima request generasi lokal, masing-masing 90 detik; response maksimal 256 KiB, 12 usulan, dan 3.072 token keluaran. Batas session pencarian tetap berlaku. `enrich` menyediakan `-timeout`, `-max-requests`, dan `-ollama-url`. Setiap generasi juga mencoba membaca metadata lokal `/api/tags` dengan batas tiga detik dalam deadline yang sama; metadata ini tidak dihitung sebagai request generasi.

Model memilih ID potongan pendek seperti `s1` melalui `evidence_id` pada respons internal. Aplikasi memetakannya kembali ke ID observasi asli dan memasang teks potongan sebagai kutipan. Model tidak boleh menghasilkan field `quote`. Pemetaan lengkap tersimpan pada `extraction[].passages`; `input_sha256` merujuk input potongan yang benar-benar dikirim. Jika aturan dapat membuktikan hubungan yang sama, kutipan difokuskan ke rentang hubungan tersebut dengan catatan koreksi. Mekanisme ini menghindari kutipan karangan dan salah ketik ID panjang, tetapi tidak menjamin semua field atau interpretasi model benar.

`extraction` menyimpan hash input/output/prompt, opsi runtime, digest bobot jika tersedia, durasi, serta alasan penolakan per usulan. Status model membedakan `EMPTY`, `REJECTED_ALL`, `PARTIAL`, `SUCCESS`, `FAILED`, dan `SKIPPED`. Kegagalan layanan memiliki `error_code`; penolakan usulan memiliki `rejections[].code`, detail, dan usulan asal. `accepted` berarti lolos validator, bukan diterima operator atau terbukti benar. Koreksi yang mempunyai dukungan deterministik dicatat pada `adjustments`, termasuk pemisahan jenis hubungan, judul jabatan publik, fungsi yang diduplikasi menjadi organisasi, dan kata kerja yang keliru menjadi role. Tidak ada tahun baru yang dikarang untuk melengkapi usulan.

Tambahkan `-debug-extraction` pada `enrich` atau `extraction.save_raw_response: true` pada konfigurasi untuk menyimpan respons mentah lokal yang dibatasi ukurannya. Default tidak menyimpan respons mentah. Informasi kutipan dan usulan yang ditolak tetap tercatat tanpa flag tersebut.

Kutipan harus benar-benar ada pada observasi milik kandidat; subjek, organisasi, jabatan dan tahun harus didukung literal oleh kutipan. Respons rusak, terpotong, berlebih, atau memakai ID potongan lain ditolak. JSON Schema membatasi subjek dan ID potongan ke input aktual. Validator menolak pengantar biografi yang jelas menyebut nama orang lebih panjang, badan kolektif yang dijadikan jabatan individu, dan periode tanpa dukungan sumber. Pemeriksaan nama ini terbatas pada pola pengantar, bukan penyelesaian identitas secara umum. Instruksi dalam halaman diperlakukan sebagai data. Validasi literal belum membuktikan hubungan semantik: kutipan dapat menyebut beberapa orang. Karena itu usulan model tetap terlihat sebagai `ollama/relations-v8`, memerlukan pemeriksaan hubungan, dan tidak otomatis mengisi ringkasan atau PES. Konfirmasi identitas kandidat juga tidak memverifikasi hubungan tersebut.

Untuk membandingkan mode secara adil, koleksi sekali lalu proses snapshot yang sama:

```powershell
.\bin\pmiyc.exe search -source web -name "Nama Lengkap" -out data/perbandingan-input.json
.\bin\pmiyc.exe enrich -input data/perbandingan-input.json -out data/perbandingan-rules.json
.\bin\pmiyc.exe enrich -input data/perbandingan-input.json -out data/perbandingan-hybrid.json -model qwen2.5:7b -debug-extraction
```

Ollama tidak menambah cakupan pencarian web pada alur ini. Periksa `relations` dan `extraction` untuk kontribusi model; ringkasan company/role tetap mengikuti rules sampai hubungan model direview. Lihat [hasil perbandingan nama baru](COMPARISON-NEW-NAMES.md).

## Review hubungan

Lihat ID, kutipan, metode, dan status review dengan `candidates`, lalu buat snapshot keputusan:

```powershell
.\bin\pmiyc.exe candidates -input data/hasil-hybrid.json
.\bin\pmiyc.exe review-relation -input data/hasil-hybrid.json -out data/hasil-reviewed.json -relation "ID_DARI_OUTPUT" -decision ACCEPT -reason "Kutipan sumber menyatakan hubungan ini secara eksplisit"
```

`ACCEPT` membuat usulan model layak dipertimbangkan dalam ringkasan sesuai aturan periode dan konflik. `REJECT` mengecualikan hubungan dari ringkasan. `PENDING` mencabut keputusan sebelumnya; usulan model kembali memerlukan review, sedangkan hubungan rules mengikuti aturan default. Riwayat keputusan, waktu, dan alasan tetap disimpan. ID hubungan stabil untuk tuple kandidat, dokumen, subjek, jenis, nilai, dan periode yang sama; keputusan ikut dipertahankan saat `enrich` dijalankan ulang. Sumber input tidak diubah. Review ini tidak mengubah identitas, status `UNVERIFIED`, atau kelayakan PES.

## Batas dan evaluasi

Pendidikan tersedia sebagai `candidates[].education_claims` dengan referensi hubungan dan periode. Perusahaan tetap khusus hubungan employment. Nama yang diamati pada halaman kandidat menjadi subjek ekstraksi sebelum nama pencarian, sehingga input nama depan dapat diperkaya dari nama lengkap yang benar-benar tercantum pada sumber. Klaim dari kandidat lain tidak dipindahkan ke akun awal. Lihat [alur penelusuran bertahap](ITERATIVE-RESEARCH.md).

- Aturan prosa terutama mencakup Indonesia dan Inggris. Pengurangan tebakan meningkatkan abstention; beberapa judul profil yang dulu menghasilkan field sekarang hanya tersimpan sebagai observasi.
- Alias/typo seperti `Jokowi dodo` tidak otomatis menjadi `Joko Widodo`. Alias yang tertulis dalam biografi membantu parsing nama kanonik, bukan penggabungan identitas antarhalaman.
- Parser mengenali tahun, tanggal ISO, serta tanggal lengkap Inggris/Indonesia, tetapi periode tersimpan tetap berpresisi tahun. Tahun akhir sampai tahun evaluasi diperlakukan historis secara konservatif; status dalam bulan/hari tertentu belum dimodelkan penuh.
- Sumber dapat salah, usang, atau saling bertentangan. Kandidat dari URL berbeda tidak digabung otomatis, dan konflik lintas kandidat belum direkonsiliasi.
- `go test ./internal/extract -run TestRelationshipChallenge -v` membandingkan kasus atribusi terhadap baseline lama. Ini corpus regresi yang diketahui saat implementasi, bukan pengukuran akurasi pada data independen.
- `TestRelationRegressionEvaluation` mengukur company, role, tuple hubungan, abstention, kesalahan periode/subjek, serta durasi pada corpus regresi yang sama untuk rules dan model lokal opsional. Lihat [VALIDATION.md](VALIDATION.md) untuk perintah dan hasil. Corpus independen tetap diperlukan untuk menilai generalisasi. Implementasi ini tidak menyatakan kesiapan Black Hat Arsenal.
