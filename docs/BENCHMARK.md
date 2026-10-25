# Evaluasi kandidat dan ekstraksi

Perintah `benchmark` menjalankan filter kandidat dan enrichment pada observasi beku. Mode rules dan hybrid menerima input identik; hash input setiap kasus disimpan agar perbandingan bisa diperiksa. Tidak ada pencarian web atau konfirmasi identitas dalam perintah ini. Ollama hanya dipanggil melalui origin loopback yang dikonfigurasi.

```powershell
.\bin\pmiyc.exe benchmark -out data/benchmark-rules.json
.\bin\pmiyc.exe benchmark -model qwen2.5:7b -timeout 30 -deadline 600 -out data/benchmark-hybrid.json
```

Model harus sudah terpasang dan server Ollama berjalan. Perintah tidak mengunduh model. `-timeout` membatasi setiap request model dan `-deadline` membatasi keseluruhan run. Pembatalan menghasilkan laporan parsial dan exit nonzero. Model gagal atau dilewati tetap dicatat; rules yang berhasil tidak dianggap bukti bahwa model berhasil. File output tidak ditimpa.

## Corpus dan label

Default corpus adalah `fixtures/benchmark-exploratory-v1.json`, berisi 24 kasus sintetis baru yang dibekukan sebelum baseline pertama. Kasus mencakup nama tunggal, nama dipendekkan, orang dengan nama sama, direktori, teks minim, negasi, pekerjaan kerabat, jabatan terpotong, periode historis, pendidikan, bahasa Indonesia, dan narasi berkata ganti.

Corpus ini tidak diberi label oleh penilai independen. Sebagian pola mirip kasus regresi yang sudah ada. Setelah hasil diperiksa dan dipakai untuk perbaikan, corpus menjadi regression set. Angkanya tidak mengukur akurasi pada orang baru di internet atau kesiapan Black Hat Arsenal.

Setiap kasus memiliki `name`, `category`, dan daftar `results`. Setiap result mempunyai URL, judul, snippet, dokumen tambahan opsional, label `keep`, serta array `expected`. Array kosong berarti tidak ada klaim yang didukung dalam cakupan evaluasi; array yang tidak diisi ditolak. `keep` menilai kelayakan sebagai kandidat untuk review, bukan apakah profil pasti milik target. Dua orang bernama sama dapat sama-sama layak ditampilkan. Klaim dan nilai diberi label per kandidat agar hubungan antarorang tidak tercampur.

`expected` memuat subject, kind, role, organization, temporal, serta tahun bila disebutkan. Institusi pendidikan hanya masuk `education`; employer hanya masuk `employment`. Label mengikuti informasi dalam dokumen, bukan pengetahuan di luar sumber. Salin corpus ke versi baru untuk memperbaiki anotasi dan dokumentasikan alasannya; jangan mengubah label hanya untuk menaikkan skor.

## Membaca laporan

- `candidate_filter`: TP/FP/FN kelayakan hasil indeks yang telah disediakan. Ini tidak mengukur recall mesin pencari, ranking, request budget, atau ketersediaan provider.
- `metrics.rules` dan `metrics.hybrid`: jumlah klaim sesuai, tambahan, dan terlewat, disertai precision/recall. Klaim identik dihitung sekali. Subject, kandidat, jenis, nilai, dan periode harus cocok; bentuk lain yang masih didukung dapat terhitung sebagai ketidaksesuaian.
- Metrik role, company, dan education dihitung terpisah. Penyebut kosong menghasilkan `null`, bukan 100%.
- `correct_abstentions`: kasus negatif yang selesai tanpa klaim. Pipeline gagal atau dibatalkan tidak dihitung sebagai abstensi benar. Kasus positif yang kosong dicatat terpisah.
- `model_status_counts`: membedakan SUCCESS, EMPTY, PARTIAL, REJECTED_ALL, FAILED, dan SKIPPED. `model_success` berarti respons berhasil diproses, termasuk respons kosong atau seluruh usulan ditolak; bukan jumlah prediksi benar.
- `runs`: hasil setiap kasus, input hash, kandidat, bukti, hubungan, trace model, klaim terlewat, klaim tambahan, dan kategori kegagalan.
- Metadata menyimpan corpus dan hash, versi ekstraktor, revisi build, perubahan lokal, versi Go, konfigurasi model, serta digest model dan prompt di trace bila tersedia.

Hybrid mencakup rules dan usulan model yang lolos grounding. Usulan model tetap `UNVERIFIED` dan belum diterima operator. Skor usulan tidak berarti company/role pada produk otomatis berubah. Pemeriksaan identitas tetap diperlukan.

## Langkah menuju evaluasi independen

Kumpulkan observasi publik dari relawan atau profil publik yang sesuai lingkup penelitian, lalu minta penilai terpisah memberi label sebelum output mesin dilihat. Simpan kegagalan pengambilan sumber, bukan hanya pencarian yang berhasil. Pisahkan set pengembangan dan set uji; jalankan set uji setelah perubahan dibekukan. Untuk mengukur pencarian langsung, tambahkan daftar profil rujukan yang diketahui dan catatan provider, query, waktu, budget, serta hasil pengambilan. Evaluasi snapshot saat ini belum menyediakan ukuran tersebut.

Lihat [hasil baseline dan perbaikan pertama](BENCHMARK-BASELINE.md).
