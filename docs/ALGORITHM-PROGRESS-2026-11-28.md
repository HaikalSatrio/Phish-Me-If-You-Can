# Progres algoritma, 28 November 2026

Perubahan ini memperbaiki pengumpulan field dan ekstraksi pada repository lokal. Scope pengujian adalah fixture sintetis, mock transport HTTP, dan server test loopback. Dasar izin: permintaan pemilik workspace untuk mengubah dan menguji project. Tidak ada pencarian terhadap orang nyata, eksploitasi, atau pengumpulan target eksternal. Target baru menunggu penetapan scope, dasar izin, dan aturan pengujian.

## Perubahan yang diuji

| Area | Sebelumnya | Perilaku baru | Bukti |
|---|---|---|---|
| Merge profil | Scalar company/location/email/website dapat hilang saat hasil digabung | Field terisi dipertahankan; nilai berbeda disimpan sebagai fakta terpisah beserta provenance sumbernya dan field ringkasan yang konflik dikosongkan | `TestMergeProfilePreservesScalarFieldsAndConflictProvenance` |
| GitHub HTML | Jalur halaman menyimpan banyak informasi hanya sebagai teks | `github-html-v3` memetakan field eksplisit di area profil dengan metadata URL/type/username yang cocok; jalur Page memakai body yang sama | `TestGitHubHTMLMapsExplicitDetailsWithoutExtraRequests` |
| Salah atribusi field | Belum ada pemetaan khusus yang dibatasi area detail | Repository, identitas tersembunyi, marker akun lain, link lokal/script, dan konflik employer diuji sebagai kontrol | `TestGitHubHTMLDetailsRequireProfileMarkers`, `TestGitHubHTMLRejectsHiddenAndUnsafeAttributes`, `TestGitHubHTMLConflictingFieldsRemainSeparateClaims` |
| Benturan nama | Semua kata nama di judul dapat berasal dari orang dengan nama depan tambahan | Format judul profil yang dikenal difilter bila query muncul setelah nama depan lain; alias eksplisit, middle name, honorific dan petunjuk akun tetap dapat direview | Corpus `benchmark-person-filter-v1.json`, `TestLeadingNameFilterKeepsHonorificsAndExactAccountClues` |
| Pendidikan | Pola `studies Computer Science at ...` terlewat | `relations-v12` mengekstrak pernyataan studi eksplisit dengan jurusan dan institusi yang dibatasi, tanpa menjadikannya pekerjaan | `TestExplicitStudyStatements`: 6 positif dan 8 kontrol negatif |
| Relasi antar-kalimat | `joined ... . She works there as ...` terlewat | Dua kalimat bersebelahan dalam paragraf yang sama dapat mengikat employer dan role; pergantian orang, pendamping, negasi, pertanyaan dan paragraf berbeda ditolak | `TestAdjacentEmploymentUsesOnlyUnambiguousLocalContext`: 3 positif dan 12 kontrol negatif |

Status perubahan di atas: **VERIFIED melalui test lokal**, bukan verifikasi kebenaran klaim profil di dunia nyata. Penerimaan kandidat filter tetap berbeda dari penerimaan identitas; semua kandidat koleksi tetap membutuhkan review.

## Perbandingan pada input identik

| Corpus | Sebelum | Sesudah |
|---|---|---|
| Filter sintetis baru, 10 kasus | TP 5, FP 3, FN 0 | TP 5, FP 0, FN 0 |
| Ekstraksi sintetis lama, 24 kasus | Klaim TP 14, FP 0, FN 2 | Klaim TP 16, FP 0, FN 0 |
| Abstention benar pada corpus lama | 11/11 | 11/11 |

TP/FP/FN dinilai terhadap label fixture. Corpus baru ditulis untuk pengembangan perubahan ini, bukan held-out. Corpus lama juga sudah menjadi regression set. Tidak ada klaim peningkatan akurasi identitas, recall web, atau generalisasi ke populasi nyata. Dua kasus lama yang sebelumnya terlewat, pendidikan dalam prosa dan relasi dua kalimat bersebelahan, kini tertangkap. General pronoun resolution tetap tidak didukung; kelulusan fixture tidak membuktikan semua bentuk bahasa sudah tertangani. Latensi benchmark lokal tidak dipakai sebagai klaim percepatan.

[Ringkasan mesin beserta hash dan referensi artefak](../benchmarks/results/2026-11-28-algorithm-regression.json). Laporan mentah disimpan di direktori lokal `data/algorithm-review-20261128-011525/` dan tidak masuk Git.

## Validasi dan reproduksi

Seluruh `go test ./...`, `go vet ./...`, race test untuk storage/collector/extract/search, dan enam test Python lulus di lingkungan lokal. Test field GitHub dan aturan ekstraksi baru juga lulus langsung di Windows. Model dan collector live tidak diaktifkan. Binary Windows hasil build dipakai untuk kedua replay benchmark; laporan mencatat hash corpus yang sama sebelum dan sesudah.

```powershell
go test ./...
go vet ./...
go test -race ./internal/storage ./internal/collector ./internal/extract ./internal/search
python -m unittest discover -s scripts -p 'test_*.py' -v
go build -o bin/pmiyc.exe ./cmd/pmiyc
.\bin\pmiyc.exe benchmark -corpus fixtures/benchmark-person-filter-v1.json -out data/filter-run-baru.json
.\bin\pmiyc.exe benchmark -out data/extraction-run-baru.json
```

Gunakan nama output baru. Go/race checks pada run ini memakai WSL Go 1.26.4; replay dijalankan dengan executable Windows. CLI membatasi replay ini pada input fixture dan tidak membuka URL hasil.

## Leads board dan batas yang belum selesai

| Prioritas | Lead | Status dan syarat kelanjutan |
|---|---|---|
| 1 | Evaluasi eksternal pada akun milik sendiri/test account | Belum dijalankan; perlu scope, dasar izin, teknik yang diizinkan dan batas request |
| 2 | Generalisasi pronoun dan relasi antar-kalimat | Pola dua kalimat sempit sudah ditangani; bentuk lain tetap perlu corpus terpisah dan pengikatan subjek yang menolak pergantian orang |
| 3 | Coverage pendidikan dan employer lintas bahasa | Pola baru masih sempit; perlu corpus berlabel terpisah dengan kasus absen/tidak pasti, periode historis dan banyak orang |
| 4 | API enrichment akun yang sudah diketahui | Belum diintegrasikan ke discovery web; perubahan sekarang memakai HTML yang telah diambil tanpa request tambahan |
| 5 | Lock akses setelah proses mati paksa | Pemulihan manual masih berlaku; penghapusan otomatis berdasarkan umur saja dapat membuka dua writer bersamaan |
| 6 | Reproduksi layanan pencarian | SearXNG masih memakai image `latest`; perlu pin digest dari image yang benar-benar diuji, lalu evaluasi ulang |
| 7 | Privasi penyimpanan | Snapshot lokal belum terenkripsi; pengaturan retensi/redaksi belum tersedia |

Ini satu tahap perbaikan terukur. Daftar tersebut tetap terbuka; kelulusan test tidak berarti seluruh kelemahan project telah diperbaiki.
