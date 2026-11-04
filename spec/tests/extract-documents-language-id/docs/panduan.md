# Panduan Cabang

Panduan untuk petugas sebuah cabang perpustakaan. Edisi 4.

## 1 Anggota

1.1 Seorang anggota dikenali dari nomor kartunya. Cabang mencatat nama
lengkap dan nomor telepon setiap anggota, yang merupakan data pribadi.

| No. | Field | Tipe | Wajib | Sensitivitas |
|---|---|---|---|---|
| 1 | Nomor kartu | Teks | ya | internal |
| 2 | Nama lengkap | Teks | ya | pribadi |

1.2 Petugas sebaiknya memeriksa kartu pada setiap kunjungan.

## 2 Peminjaman

2.1 Masa pinjam adalah 14 hari. Ia harus dikembalikan ke cabang yang meminjamkannya.

| Field | Tipe | Wajib |
|---|---|---|
| Tanggal pinjam | Tanggal | ya |
| Denda | Uang | tidak |
| Nomor telepon | Teks | tidak |

2.2 Cabang harus mengirim pengingat sebelum tanggal jatuh tempo:

- lewat SMS, dan
- lewat pos, jika anggota tidak punya telepon.

## 3 Jam buka

Cabang buka pukul 09.00.

| Hari | Jam |
|---|---|
| Senin | 09.00 sampai 17.00 |

```
open 09:00
```
