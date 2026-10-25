# Baseline 25 Oktober 2026

Corpus: 24 kasus sintetis, 25 hasil indeks yang disediakan, dan 16 hubungan berlabel. SHA256 corpus yang dikanonisasi melalui JSON: `247c342d37d5b4a7096f8856c4860777d20b0e39ab0d95fab6085db435042c0d`. Semua hash input kasus sama pada baseline dan run setelah perbaikan. Corpus dan label tidak diubah setelah hasil pertama dilihat.

Model lokal: `qwen2.5:7b`, digest `845dbda0ea48ed749caafd9e6037047aa19acfcfd82e704d7ca97d631a0b697e`, temperature 0, num_ctx 8192, num_predict 3072. Ada 21 request generasi pada setiap run hybrid, tanpa FAILED atau SKIPPED. Respons kosong dan respons dengan seluruh proposal ditolak tetap masuk jumlah request. Waktu di bawah adalah satu pengukuran lokal, bukan benchmark performa perangkat.

| Mode | Klaim sesuai | Tambahan di luar label | Terlewat | Precision ketat | Recall ketat | Abstensi sesuai | Waktu |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| Rules v7 | 14 | 0 | 2 | 100% | 87,5% | 11/11 | 48 ms |
| Rules + Qwen v7 | 15 | 5 | 1 | 75% | 93,75% | 9/11 | 40.669 ms |
| Rules v8 | 14 | 0 | 2 | 100% | 87,5% | 11/11 | 37 ms |
| Rules + Qwen v8 | 15 | 3 | 1 | 83,33% | 93,75% | 10/11 | 26.729 ms |

Precision dan recall memakai pencocokan tuple kandidat, subject, kind, role, organization, temporal, dan tahun. Angka tambahan di luar label tidak identik dengan jumlah kebohongan faktual: hubungan redundan, pilihan batas field, dan perbedaan periode juga dihitung. Tidak ada identitas yang dikonfirmasi otomatis. Filter kandidat mempertahankan 21 hasil yang dilabeli layak dan menolak 4 hasil yang dilabeli tidak layak; ini tidak mengukur keberhasilan menemukan profil di web.

## Temuan dan perbaikan

1. `relative-profession`: Qwen memindahkan profesi suami ke pemilik bio. Grounding v8 menolak klaim ketika role atau organisasi yang diusulkan berada pada klausa eksplisit tentang kerabat atau teman. Tes mencakup Inggris, Indonesia, dan profesi pemilik yang berbeda dari profesi kerabat.
2. `student-education-label`: Qwen memakai `Undergraduate Computer Science Student` sebagai institusi pendidikan. Grounding v8 menolak institusi yang berupa judul role/student. Pendidikan yang sudah didukung label institusi tetap dihasilkan rules.
3. `narrative-pronoun`: rules melewatkan hubungan pada kalimat berikutnya dengan kata ganti. Qwen melengkapinya sesuai label. Ini kontribusi model yang nyata pada corpus ini.
4. `education-in-prose`: Qwen menemukan institusi yang benar tetapi memakai role `studies Computer Science` dan periode headline, sementara label menetapkan role kosong dan present_assertion. Pada metrik institusi pendidikan, recall hybrid mencapai 2/2; pada tuple lengkap masih ada satu klaim terlewat dan satu tambahan.
5. `shortened-last-name`: Qwen menambahkan occupation yang redundan terhadap employment yang sudah benar. Tidak ada institusi atau role baru yang direkayasa, tetapi tuple tambahan tetap dihitung FP.
6. `truncated-employer`: Qwen mengambil role yang terlihat sebelum employer terpotong. Corpus menetapkan abstensi untuk headline terpotong; ketidaksesuaian ini menunjukkan perlunya kebijakan anotasi yang ditinjau terpisah. Label awal tetap dipertahankan untuk perbandingan.

Perbaikan v8 menolak dua kesalahan semantik yang jelas. Penolakan tetap memiliki kode diagnosis dan kutipan; tidak ada penerimaan hubungan otomatis. Perbaikan ini terbatas pada pola yang dikenali, belum menjadi pemahaman atribusi secara umum.

## Artefak dan interpretasi

- [Laporan lengkap v7](../benchmarks/results/2026-10-25-v7.json)
- [Laporan lengkap v8](../benchmarks/results/2026-10-25-v8.json)
- [Corpus dan label](../fixtures/benchmark-exploratory-v1.json)
- [Cara menjalankan dan definisi metrik](BENCHMARK.md)

Hasil ini mendukung penggunaan Ollama sebagai pengusul hubungan yang direview. Data ini belum cukup untuk menyatakan bahwa hybrid lebih akurat secara umum. Corpus dibuat oleh implementer, sebagian memakai pola yang dikenal, dan sekarang sudah dipakai untuk perbaikan. Pengukuran berikutnya memerlukan anotator terpisah serta snapshot nyata yang belum digunakan untuk menyetel rules atau prompt. Jangan menggunakan angka ini sebagai klaim akurasi OSINT di internet.
