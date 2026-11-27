# Koreksi implementasi yang sudah ada

Perbaikan ini berfokus pada perilaku yang keliru di implementasi sebelumnya. Tidak ada fitur pengumpulan baru, target eksternal, atau model live yang dijalankan. Scope: kode lokal, fixture sintetis, dan HTTP mock/test loopback, sesuai permintaan pemilik workspace.

| Kesalahan | Koreksi | Test regresi |
|---|---|---|
| URL perusahaan di dalam `worksFor` dianggap website pribadi | Field perusahaan/lokasi tidak meneruskan tautan bersarang ke field kontak pribadi | `TestGitHubNestedOrganizationURLIsNotPersonalWebsite` |
| Teks anak elemen tersembunyi ikut terbaca sebagai nama/username | Parser identitas memakai teks yang terlihat saja, termasuk ketika markup tersembunyi berada di dalam elemen nama | `TestGitHubIdentityTextExcludesHiddenChildren` |
| Merge nama mengabaikan provenance untuk nilai spesifik | Menggunakan `SourceForFact`, sehingga URL, request URL, timestamp, hash dan rule sumber tetap utuh | `TestMergedNameKeepsExactValueProvenance` |
| Kata `known as` di bagian lain cuplikan meloloskan orang yang salah | Pengecualian alias harus menghubungkan nama pada judul dengan nama query; alias orang lain, negasi dan pertanyaan tidak cukup | `TestUnrelatedAliasMentionDoesNotOverrideLeadingNameCollision` |
| Pemisahan kalimat membuang `?`, sehingga pertanyaan menjadi klaim pendidikan | Ekstraksi pernyataan eksplisit mempertahankan penanda pertanyaan dan melewatkannya, tanpa membuang pernyataan berikutnya | `TestExplicitStudyStatements/question`, `TestStatementAfterQuestionRemainsEligible` |

Kesalahan URL perusahaan dan pertanyaan berhasil direproduksi sebagai test gagal sebelum koreksi. Semua test di atas lulus setelah koreksi, termasuk eksekusi native Windows. Versi perilaku sekarang adalah `relations-v13`, `github-html-v4`, `devto-html-v3`, dan `public-page-v8`; versi lama pada laporan historis tetap dipertahankan.

## Perbandingan input yang sama

Pada enam kasus baru dalam `fixtures/benchmark-existing-repairs-v1.json`:

| Ukuran | Sebelum, v12 | Sesudah, v13 |
|---|---:|---:|
| Kandidat benar dipertahankan | 4 | 4 |
| Kandidat salah dipertahankan | 2 | 0 |
| Kandidat benar terlewat | 0 | 0 |
| Klaim benar | 2 | 2 |
| Klaim salah | 2 | 0 |
| Klaim benar terlewat | 0 | 0 |

Kontrol corpus lama juga tetap lulus: filter 10 kasus menghasilkan TP 5/FP 0/FN 0; ekstraksi 24 kasus menghasilkan TP 16/FP 0/FN 0, dengan abstention benar 11/11. Ini regression set development, bukan held-out atau ukuran ketepatan identitas nyata. Angka baru tidak boleh dibandingkan langsung dengan angka corpus lama yang berbeda.

[Ringkasan mesin beserta hash](../benchmarks/results/2026-11-28-existing-repairs.json) mencatat kedua executable, kode yang diuji, corpus, dan laporan. Laporan mentah tetap berada di direktori lokal `data/existing-repairs-20261128-015936/` yang tidak masuk Git. Laporan v12 sebelumnya tidak diubah.

## Validasi

- `go test ./...` dan `go vet ./...` lulus melalui WSL.
- Race checks collector/extract/search lulus.
- Seluruh enam test Python reporter lulus.
- Test koreksi collector dan ekstraksi lulus langsung di Windows.
- Build Windows diperbarui di `bin/pmiyc.exe`.

Reproduksi dengan nama output baru:

```powershell
go test ./...
go vet ./...
go test -race ./internal/collector ./internal/extract ./internal/search
python -m unittest discover -s scripts -p 'test_*.py' -v
go build -o bin/pmiyc.exe ./cmd/pmiyc
.\bin\pmiyc.exe benchmark -corpus fixtures/benchmark-existing-repairs-v1.json -out data/repairs-run-baru.json
```

Kelulusan ini terbatas pada kasus yang diuji. Proposal fitur baru dan pekerjaan lanjutan pada laporan progres sebelumnya tidak termasuk dalam koreksi ini.
