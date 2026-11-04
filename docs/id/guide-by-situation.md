# SpecArch menurut situasi

Satu tempat yang mengatakan fitur mana dipakai pada situasi nyata yang
mana, untuk orang dan untuk agen. Setiap situasi menyebutkan kapan ia
terjadi, apa yang dijalankan, memperlihatkan contoh singkat beserta
keluaran yang dicetak program, menyebutkan apa yang perlu diperhatikan,
dan menunjuk dokumen yang lebih dalam. `specarch help situations`
mencetak situasi yang sama sebagai indeks singkat yang dibangun ke dalam
program; `specarch help <topik>` dan `specarch help <perintah>` mencetak
setiap bidang dan setiap perintah secara lengkap. Panduan yang sama dalam
bahasa Inggris adalah `docs/guide-by-situation.md`, dengan id situasi yang
sama.

Contoh-contohnya dijalankan pada spesifikasi di repositori ini: contoh
peminjaman perpustakaan (`examples/library-lending`, lengkap, dengan
catatan di sampingnya), meja peminjaman (`examples/lending-desk`, dibaca
dari sebuah manual dan kode yang saling bertentangan, dengan pertanyaan
terbuka) dan pemantau rilis (`examples/release-watcher`, dua daemon di
bawah systemd), serta pada kasus konformansi di bawah `spec/tests/`.
Keluaran perintah diperlihatkan sebagaimana program mencetaknya; daftar
yang panjang dipotong, dengan satu baris yang menyebutkan apa yang
menyusul, dan path absolut yang dicetak program dipendekkan menjadi path
dari folder kerja. Program berbicara dalam bahasa Inggris pada setiap
bahasa panduan ini.

## Dua kenyataan yang diperhitungkan setiap situasi

Kebutuhan tidak pernah tetap. Orang melewatkan hal-hal, mengatakan sesuatu
yang bukan maksud mereka, dan berubah pikiran begitu melihat sistemnya.
Karena itu spesifikasi di sini tidak pernah ditolak karena belum lengkap:
ia tetap dibaca, setiap celah menjadi masalah atau pertanyaan terbuka pada
entrinya, dokumen dan draf tetap ditulis darinya, dan sebuah perubahan
adalah catatan beserta sebabnya. Pengguna datang terlambat. Orang yang
akan memakai sistem datang pada uji terima pengguna, bukan pada rapat
kebutuhan, dan di sana meminta perubahan. Karena itu setiap hasil yang
dibangkitkan membawa jejak kembali ke elemen asalnya, keberatan pada uji
terima adalah permintaan perubahan yang menyebut elemen itu, dan validator
memperingatkan peran yang belum pernah didengar sebelum uji terima,
sehingga pengguna yang absen setidaknya ditanya.

| Situasi | Id |
|---|---|
| Memulai sistem baru | `new-system` |
| Mewawancarai orang | `interviewing` |
| Menangkap dari dokumen pelanggan | `customer-documents` |
| Menangkap dari kode dan basis data yang ada | `existing-code` |
| Register tabular | `registers` |
| Daemon dan unit layanan | `daemons` |
| Mencatat percakapan dan siapa yang hadir | `conversation-record` |
| Kebutuhan yang dirinci | `requirement-detailed` |
| Membangkitkan draf sebelum penyetujuan untuk diperlihatkan ke pengguna | `draft-before-approval` |
| Menyetujui spesifikasi | `approving` |
| Membangun sistem dari spesifikasi | `building` |
| Menjaga spesifikasi dan kode tetap sama di CI | `ci` |
| Perubahan kecil | `small-change` |
| Perubahan besar di akhir siklus hidup | `large-change-late` |
| Pengguna yang absen berkeberatan pada uji terima | `absent-user-objects` |
| Menyetujui lagi, dengan diff | `approve-again` |
| Rilis dan catatan rilis | `release` |
| Rilis SpecArch memindahkan format | `formats-move` |
| Agen yang mengemudikan SpecArch | `agents` |

<!-- situation: new-system -->
## Memulai sistem baru

**Kapan terjadi.** Belum ada apa-apa: tidak ada kode, tidak ada basis
data, tidak ada dokumen lama. Seseorang punya gagasan dan beberapa orang
yang tahu apa yang mereka butuhkan.

**Apa yang dijalankan.**

    specarch init --profile lean notes
    specarch validate notes
    specarch gaps notes
    specarch document techspec --out notes/docs notes

`specarch init` menulis spesifikasi terkecil yang lolos validasi, dan
tidak lebih: setiap kunci di luar itu adalah keputusan yang belum diambil
siapa pun. Profil lean menjadikan tes yang belum ada sebagai peringatan,
sehingga alat kecil atau daemon internal bisa divalidasi, didokumentasikan
dan diperlihatkan sebelum tesnya ditulis; pada profil full, bawaannya,
setiap tes yang tersirat dari rancangan adalah kesalahan sampai ditulis.
`specarch help start` memperlihatkan entitas dan operasi pertama serta
apa yang dilonggarkan lean; `specarch help examples` adalah contoh dari
folder kosong sampai berkas yang dibangkitkan; `specarch help format`
adalah tata letak berkas; `specarch help conventions` kata kuncinya.

**Contoh.** Pada folder baru:

    $ specarch init --profile lean notes
    notes/specarch.yaml
    specarch init: next, run specarch validate notes after each edit; specarch help start shows a first entity and operation that validate
    $ specarch validate notes
    notes/specarch.yaml:7:12: warning: /info/profile: profile: the specification is on the lean profile, so a test specarch derive would write and nobody has is a warning, not an error (test_golden_missing, test_case_missing); every other rule is as on full. Write profile: full, or remove the key, to make them errors, and run specarch derive to write the tests [profile@specarch.yaml#/info/profile]
    specarch: 1 input checked: 0 errors, 1 warning

**Yang perlu diperhatikan.** Tulis pemangku kepentingan lebih dulu,
menurut peran, karena setiap pertanyaan menyebut siapa yang memutuskan dan
setiap catatan menyebut siapa yang berbicara; validate memberi warning
`stakeholders_unconfirmed`, dan setiap dokumen membawa banner, sampai ada
peran yang mengonfirmasi daftarnya sudah lengkap lewat
`stakeholdersComplete` (by, date, dan source kalau ada), supaya peran yang terlewat
terlihat sebelum acceptance test, bukan saat UAT berjalan. Lima hal yang dilewatkan
draf pertama adalah kesalahan pada setiap profil: kunci utama entitas,
ringkasan operasi, deskripsi setiap respons, satu izin yang dibutuhkan
operasi (akses tertutup kecuali diberikan) dan bilangan bulat yang
dibatasi. Sesuatu yang belum diketahui adalah pertanyaan terbuka di bawah
`questions`, bukan placeholder di dalam nilai.

**Lebih dalam.** `docs/principles.md` (untuk apa SpecArch dibuat),
`docs/stages.md` (tujuh tahap), `docs/conventions.md` (pohon dan kata
kunci).

<!-- situation: interviewing -->
## Mewawancarai orang

**Kapan terjadi.** Spesifikasi punya pertanyaan terbuka, dan seseorang
yang bisa menjawabnya memberi Anda satu jam. Atau seorang agen sedang
berbincang dengan analis dan analis sedang berbincang dengan pelanggan.

**Apa yang dijalankan.**

    specarch gaps --next examples/lending-desk
    specarch decide Q-3 --by desk-manager --answer "<pilihan yang diambil, lengkap>" --source <wawancara> examples/lending-desk/spec
    specarch validate examples/lending-desk

`specarch gaps` mencetak setiap pertanyaan terbuka menurut tahap, siapa
yang memutuskan, apa yang ditahannya dan keluaran mana yang siap, draf
atau menunggu; `specarch document questions` menulis hal yang sama
sebagai dokumen. `gaps --next` memilih satu pertanyaan yang ditanyakan
berikutnya: must sebelum should, lalu yang menahan lebih banyak keluaran.
Prosedurnya, satu pertanyaan pada satu waktu dengan dua sampai lima
pilihan dan satu yang direkomendasikan, ada di `specarch help asking`;
`specarch help gaps` menjelaskan pertanyaan, keputusan dan penyetujuan.

**Contoh.** Meja peminjaman punya delapan pertanyaan terbuka:

    $ specarch gaps --next examples/lending-desk
    Next question in Lending Desk (examples/lending-desk/spec/specarch.yaml), ranked first of 8 open questions: Q-3 (must, decision)

    How is each requirement shown to be met?

    - Decided by: desk-manager
    - Blocks: #/requirements/LEND-1/acceptance, #/requirements/LEND-2/acceptance, #/requirements/LEND-3/acceptance, #/requirements/LEND-4/acceptance, #/requirements/LEND-5/acceptance, #/requirements/LEND-6/acceptance, #/requirements/LEND-7/acceptance
    - Holds up: techspec document, requirements document, testplan document, traceability document and html document
    - Options: none written; offer two to five
    - Insight: Neither the manual nor the code gives acceptance criteria, and the code has no tests to read them from.
    - Record the answer: specarch decide Q-3 --by desk-manager --answer "<answer>" examples/lending-desk/spec
    - Source: the root file declares no source of kind interview; declare an interview under sources for the answer to cite, and add --source with its key

    Put it to desk-manager alone, with one answer recommended and why; specarch help asking.

Manajer meja memilih; keputusan dicatat dengan teks pilihan selengkapnya,
bukan nomor yang disebut orang itu. Spesifikasi yang melacak asal menolak
keputusan tanpa sumber:

    $ specarch decide Q-3 examples/lending-desk/spec --dry-run --by desk-manager --answer "Each requirement is shown met by one test of the operation it names."
    specarch decide: examples/lending-desk/spec tracks origin, so the decision must cite where the answer came from; record the answer with specarch log and give its ID with --record, or declare it under sources in specarch.yaml, as an interview with its date, and give it with --source

**Yang perlu diperhatikan.** Jangan pernah menjawab pertanyaan sendiri,
dan jangan pernah memilih di antara dua sumber yang bertentangan: orangnya
yang memilih. Jalankan `gaps --next` lagi setelah setiap jawaban; satu
jawaban menyelesaikan atau membuka pertanyaan lain. Bila orang yang Anda
ajak bicara bukan peran yang disebut pertanyaan, teruskan ke orang yang
tepat. Jawaban yang diberikan lewat chat atau telepon adalah catatan
komunikasi lebih dulu (`conversation-record`), dan `decide --record`
mengutipnya.

**Lebih dalam.** `docs/refinement.md` (pertanyaan terbuka, asal,
keputusan yang menutup pertanyaan), `docs/questions.md` (pertanyaan
terbuka SpecArch sendiri, dibangkitkan).

<!-- situation: customer-documents -->
## Menangkap dari dokumen pelanggan

**Kapan terjadi.** Pelanggan menyerahkan apa yang mereka punya: manual
dalam Word, aturan dalam PDF, setumpuk slide, spreadsheet berisi kolom,
utas email berisi keputusan. Belum ada spesifikasi, dan dokumen-dokumen
itu di beberapa tempat saling bertentangan.

**Apa yang dijalankan.**

    specarch extract documents --out trees/manual sources/manual.md
    specarch extract documents --out trees/rules sources/rules.pdf
    specarch merge --out spec trees/manual trees/rules
    specarch validate spec
    specarch gaps spec
    specarch document comparison --out docs spec

`extract documents` membaca satu berkas Markdown, PDF, Word (.docx),
PowerPoint (.pptx), Excel (.xlsx) atau email (.eml) menjadi spesifikasi
parsial, setiap elemen mengutip klausanya: nomor bagian, halaman, slide,
rentang sel atau lampiran. `--vocabulary` menyebut kata-kata yang dipakai
dokumen untuk jenis kalimatnya (must, shall) dan `--language` bahasanya.
`extract outline` mencatat berkas-berkas suatu permukaan yang belum ada
pembacanya, masing-masing sebagai klausa tanpa elemen, sehingga `gaps`
mendaftarnya sebagai tidak menghasilkan apa-apa, bukan hilang. `specarch
merge` menggabungkan pohon-pohon itu, menyimpan kedua sisi suatu
pertentangan sebagai pertanyaan, dan `document comparison` memperlihatkan
dokumen lama terhadap spesifikasi yang sudah dimurnikan: apa yang
dipertahankan dari setiap klausa, apa yang disimpulkan, apa yang
ditambahkan sebuah keputusan, dan klausa yang tidak dikutip apa pun.

**Contoh.** Manual meja peminjaman:

    $ specarch extract documents --out trees/manual examples/lending-desk/sources/manual.md
    commit 85f752f8ddf25f3f53c9196d923c07ac54521429: the last change to examples/lending-desk/sources/manual.md
    counted 7 headings, 8 numbered paragraphs, 17 sentences (4 commitments) and 1 table (1 table of fields): every ATX heading, every paragraph that starts with a section number such as 3.1, every sentence of a paragraph or list item, and every table, outside fenced code
    wrote 4 requirements, 1 entity and 2 questions: one per commitment that names its subject, one per table of fields, one per thing the document does not say, and one per thing the meta-model cannot hold
    derived 0 draft tests under tests, as specarch derive writes them: the success of each subject the design gives one, and each chosen case no test covers

Setiap elemen yang ditulis membawa `origin: stated` beserta klausa
asalnya; apa yang tidak dikatakan manual menjadi pertanyaan, bukan
tebakan.

**Yang perlu diperhatikan.** Sumbernya harus sudah di-commit: pembaca
mencatat commit yang dibacanya, dan menolak berkas dengan perubahan yang
belum di-commit. PDF terenkripsi, berkas yang bukan formatnya menurut
ekstensi, atau dokumen dalam bahasa yang tidak dicakup kosakata mana pun
berakhir dengan status keluar 1 beserta alasannya; `specarch help extract`
mendaftarnya. Tidak ada yang dikarang untuk mengisi celah: pertanyaan
adalah pekerjaan yang tersisa, dan `interviewing` adalah cara menutupnya.

**Lebih dalam.** `docs/extraction.md` (permukaan dan pembacanya),
`docs/from-sources.md` (prosedur yang diikuti agen, dokumen lebih dulu),
`docs/vocabularies.md`.

<!-- situation: existing-code -->
## Menangkap dari kode dan basis data yang ada

**Kapan terjadi.** Sistem sudah berjalan, dan spesifikasinya adalah
kodenya. Penulisan ulang, serah terima atau audit membutuhkan spesifikasi
as-built, dan dokumen yang ada mengatakan hal lain.

**Apa yang dijalankan.**

    specarch extract database --out trees/database sources/catalogue.json
    specarch extract router --out trees/router sources/routes.json
    specarch extract openapi --out trees/openapi sources/openapi.yaml
    specarch extract permissions --out trees/permissions sources/permissions.json
    specarch extract go --out trees/go sources/code
    specarch facts javascript sources/web sources/facts/web.json
    specarch extract pages --out trees/pages sources/web/app --facts sources/facts/web.json
    specarch extract workflows --out trees/workflows sources/workflows/write-off.bpmn
    specarch extract gateway --out trees/gateway sources/gateway.yaml
    specarch merge --out spec trees/*

Satu pembaca per permukaan: `database` membaca dump katalog, `router`
tabel rute yang dicetak router yang sudah dibangun, `openapi` dokumen
OpenAPI, `permissions` tabel yang dibaca pemeriksaan yang berjalan,
`pages` folder router berbasis sistem berkas, `workflows` BPMN 2.0,
`gateway` tabel sebuah gateway, dan `go` mengurai sumber Go pada dxlib,
`net/http` dan router-router yang umum. Swift, JavaScript dan Dart dibaca
lewat dump fakta kode yang ditulis `specarch facts <bahasa>` dari folder
yang sudah di-commit, dibaca oleh `extract swift`, `extract javascript`
dan `extract dart`. `examples/lending-desk/extract.sh` menjalankan setiap
pembaca pada contoh itu dan menggabungkannya.

**Contoh.** Penggabungan delapan pohon meja peminjaman:

    $ sh examples/lending-desk/extract.sh out
    ...
    extract.sh: wrote out/spec; specarch: 1 input checked: 0 errors, 24 warnings, 39 open questions
    $ head -4 out/merge.txt
    tree trees/manual: Lending desk manual, on the documents side: 6 elements and 2 questions
    tree trees/openapi: Lending desk API, on the documents side: 11 elements and 10 questions
    tree trees/database: Database of examples/lending-desk/sources/code/migrations, on the code, printed side: 6 elements and 4 questions
    tree trees/router: Routes of examples/lending-desk/sources/code, on the code, printed side: 12 elements and 15 questions

Lalu merge.txt menyebutkan, satu baris masing-masing, pertanyaan mana
dari satu pohon yang dijawab pohon lain (`answered: Q-2 of trees/manual,
on #/entities/Members/primaryKey, is left out; Database of
examples/lending-desk/sources/code/migrations gives it`), mana yang
ditanyakan dua pohon sekaligus (`asked twice`), dan mana yang digabungkan.

**Yang perlu diperhatikan.** Sumber yang dibaca pada satu commit terpaku
padanya: dump yang dibuat ulang dari commit lain, clone dangkal atau path
di luar repositori ditolak beserta alasannya. Di mana manual dan kode
bertentangan, keduanya tetap ada, sebagai pertanyaan untuk pemilik; agen
tidak pernah memilih. Berkas implementasi, yang khusus untuk stack,
ditulis dari kode dengan tangan (`docs/from-sources.md`, 3.4). Setelah
spesifikasi disetujui, `specarch check` menjaganya tetap sama dengan kode
yang berjalan (`ci`).

**Lebih dalam.** `docs/extraction.md`, `docs/reading-code.md` (apa yang
diambil setiap pembaca bahasa dan apa yang tetap pada pencetak),
`docs/from-sources.md`, `docs/gateway.md`.

<!-- situation: registers -->
## Register tabular

**Kapan terjadi.** Sebagian sistem adalah spreadsheet yang diisi orang:
register pinjaman per kuartal, daftar aset per lokasi, satu berkas per
periode, dengan templat dan lembar petunjuk di suatu tempat. Kolomnya
adalah entitas yang belum pernah ditulis siapa pun.

**Apa yang dijalankan.**

    specarch extract tables --out spec docs/loans-2026-q1.csv docs/loans-2026-q2.csv
    specarch extract tables --out spec --entity Loan --template docs/loans-template.xlsx --instructions docs/loans-instructions.xlsx docs/loans-2026-q1.csv
    specarch generate register --out registers spec
    specarch check spec docs/loans-2026-q3.csv

`extract tables` membaca berkas CSV, TSV dan Excel dari satu register
menjadi satu entitas yang kolomnya adalah kolom-kolom register, tiap
tipe, wajib atau tidak dan nilainya disimpulkan dari setiap nilai setiap
berkas dan dikutip pada berkas, lembar dan kolomnya; templat dan petunjuk
memberi pernyataan kolom-kolomnya. `generate register`, plug-in
`specarch-gen-register`, menulis kembali register sebagai templat yang
diisi orang, dan `specarch check` membandingkan berkas yang terisi dengan
entitasnya.

**Contoh.** Berkas satu kuartal, ditulis dengan byte order mark, akhir
baris CRLF dan titik koma:

    $ specarch extract tables --out spec docs/loans-2026-q1.csv
    commit 2ccb823f3b898bfa12e0b7870ea3df72a8f08169: the last change to loans-2026-q1.csv
    dialect loans-2026-q1.csv: UTF-8 with a byte order mark, CRLF line ends, delimiter ';', no quoted field
    counted 1 file, 1 table, 12 rows and 6 columns: every file given, every register sheet or file as a table, its first row the header and every other row a record, and every header that names a field as a column, once per register
    entity Loans2026Q1: 6 fields from 12 rows in loans-2026-q1.csv; candidate keys loanId, dueDate
    wrote 1 entity and 2 questions: one per entity's primary key, one per column whose values do not settle its type, whether it is required or its values, or that some files or the template lack, one per header that names no field, and one per thing the files hold that is not read
    derived 0 draft tests under tests, as specarch derive writes them: the success of each subject the design gives one, and each chosen case no test covers

**Yang perlu diperhatikan.** Kunci utama adalah pertanyaan must yang
menyebut kolom-kolom kandidat, karena nilai saja tidak bisa mengatakan
kolom mana yang menjadi kunci. Kolom yang nilainya terbaca sebagai dua
jenis tetap string dengan pertanyaan pada tipenya, mengutip sel-sel yang
lebih jarang. Berikan berkas-berkas satu register bersama, atau `--entity`
menamainya; templat yang diberikan pada satu jalannya beberapa register
ditolak.

**Lebih dalam.** `docs/extraction.md` (register), `docs/generators.md`
(target register), `specarch help extract`.

<!-- situation: daemons -->
## Daemon dan unit layanan

**Kapan terjadi.** Sistem tidak punya pengguna di layar: ia adalah proses
yang dijalankan dan diawasi pengelola layanan, pekerjaan yang dijalankan
timer setiap malam, pemantau yang menanyai registri. Tak ada yang
memanggilnya, jadi tak ada di dalamnya yang membutuhkan izin, dan yang
dibutuhkannya adalah aturan restart, sinyalnya, status keluarnya,
setelannya dan sebuah monitor.

**Apa yang dijalankan.**

    specarch extract systemd --out spec sources/units/release-watcher.service sources/units/release-prune.service sources/units/release-prune.timer
    specarch validate spec
    specarch document operations --out docs spec
    specarch document deployment --out docs spec

Sebuah proses ditulis di bawah `processes`: cara ia berjalan (selalu,
terjadwal atau sesuai permintaan), pekerjaan yang dijalankannya, setelan
yang dibacanya, sinyal dan status keluar yang dihormatinya, kapan ia
di-restart dan batas setelah mana ia ditinggalkan, kesehatannya dan
monitornya. `extract systemd` membaca unit `.service` dan `.timer` ke
dalamnya, mengutip setiap baris. Panduan operasi menulis daftar periksa
harian, setiap monitor beserta tanggapan ketika sasarannya terlewat, dan
langkah backup, restore dan rollback; panduan deployment lingkungan dan
host-nya.

**Contoh.** Tiga unit pemantau rilis:

    $ specarch extract systemd --out spec examples/release-watcher/sources/units/release-watcher.service examples/release-watcher/sources/units/release-prune.service examples/release-watcher/sources/units/release-prune.timer
    commit f4ad62a18e50e825f89506a210ee6c5e4745d3bf: the last change to examples/release-watcher/sources/units/release-prune.service, examples/release-watcher/sources/units/release-prune.timer, examples/release-watcher/sources/units/release-watcher.service
    counted 2 service units and 1 timer unit in examples/release-watcher/sources/units/release-prune.service, examples/release-watcher/sources/units/release-prune.timer, examples/release-watcher/sources/units/release-watcher.service
    wrote 2 processes, 2 settings and 14 questions: one process per service unit, its time from its timer, one setting per variable Environment= sets, and one question per thing the units leave to the host or do not say
    not held: examples/release-watcher/sources/units/release-prune.service:6: [Service] ExecStart= says how this host runs the process, which an implementation file's deployment holds, not the design; could question Q-6
    ... 8 more lines of what the units say that the design does not hold
    not derived: 6 draft tests for process releaseWatcher, held up by Q-2, Q-3; the validator warns for them until the questions are answered

**Yang perlu diperhatikan.** Apa yang diserahkan unit kepada bawaan host
menjadi pertanyaan: batas restart bila tidak ditulis, arti setiap status
keluar, jadwal yang diberikan ekspresi kalender pada host yang jamnya UTC.
`ExecStart=`, `User=` dan `WantedBy=` adalah cara satu host
menjalankannya, yang tempatnya di deployment berkas implementasi, bukan di
rancangan. Rahasia di berkas lingkungan menjadi setelan, tidak pernah
nilainya. Profil lean cocok untuk daemon internal sampai tesnya ditulis.

**Lebih dalam.** `examples/release-watcher/README.md`,
`docs/conventions.md` (Processes), `docs/deployment.md`.

<!-- situation: conversation-record -->
## Mencatat percakapan dan siapa yang hadir

**Kapan terjadi.** Analis berbicara dengan pemangku kepentingan, lewat
rapat, chat, email atau telepon, dan melaporkannya di chat dengan kata-kata
bebas: "Saya tanya soal batasnya; ketua dewan menjawab lima pada 10:15
lewat chat". Nanti seseorang akan bertanya apa yang dikatakan, oleh siapa,
kapan, dan apakah orang yang memakai sistem pernah ada di ruangan.

**Apa yang dijalankan.**

    specarch log spec --recorded-by analyst --direction in --from library-board --to analyst --person "the board chair" --at 2026-10-31T10:15:00+07:00 --channel chat --type answer --words "Five at most." --form verbatim --question Q-1
    specarch decide Q-1 --by library-board --answer "Five at most." --record COM-1 spec
    specarch validate spec
    specarch document communications --out docs spec
    specarch document timeline --out docs spec

`specarch log` menulis satu catatan komunikasi, `records/communications/
COM-<n>.yaml`, dari opsinya saja: siapa (peran, dan label orang yang
dibuang dari dokumen untuk semua orang), kapan (instan beserta offsetnya),
bagaimana (saluran), apa (answer, question, request, information,
objection), kata-katanya dan apakah verbatim atau ringkasan, berkas bukti
di bawah `records/` beserta SHA-256-nya, dan apa yang disentuhnya. Rapat
adalah catatan sesi dengan siapa yang diundang dan hadir; sesi uji dan
deploy adalah catatan tersendiri. `document communications` adalah
riwayat per pemangku kepentingan dan per kebutuhan; `document timeline`
setiap peristiwa, dari catatan dan spesifikasi, dalam satu urutan waktu.

**Contoh.** Separuh fakta diberikan:

    $ specarch log spec --recorded-by analyst --from member --to analyst --type answer --words "five at most" --form summary
    missing --direction: ask which way it went: did the stakeholder say it to the project (in), or the project to the stakeholder (out)?; one of in, out
    missing --at: ask when it was said: the date, the time and the offset of the clock it was read on (the specification's time zone is UTC); written as 2026-10-31T10:15:00+07:00
    missing --channel: ask how it was said; one of meeting, chat, email, call, document
    missing --question: ask which question of the specification it answers, by ID
    specarch log: 4 options not given; ask the analyst what each line above asks, and run it again with every option; nothing written

Semuanya diberikan, catatan ditulis dan baris keputusannya dicetak:

    $ specarch log --direction in --from library-board --to owner --person "the board chair" --at 2026-10-31T10:15:00+07:00 --channel chat --type answer --words "Must: a release without the limit is not acceptable. It is shown met by refusing a sixth loan." --form verbatim --recorded-by owner --evidence records/evidence/COM-1-chat.txt --question Q-1 --element CASE-1 project
    records/communications/COM-1.yaml
    next: specarch decide Q-1 --by library-board --answer "Must: a release without the limit is not acceptable. It is shown met by refusing a sixth loan." --record COM-1 project

**Yang perlu diperhatikan.** Perintah ini tidak menebak apa pun: waktu
tanpa tanggal atau offset ditanyakan kembali, peran adalah salah satu
pemangku kepentingan spesifikasi dan tidak pernah nama, dan agen
menanyakan kepada analis persis apa yang dikatakan setiap baris `missing`.
Berkas bukti yang berubah setelah dicatat gagal validasi dengan hash yang
dimilikinya sekarang. Telepon tanpa notula tetap catatan, ditandai sebagai
dicatat tanpa bukti. Sejak hari pertama, validate memperingatkan
`unheard` pada pemangku kepentingan yang dibutuhkan kebutuhan yang
diterima dan tidak ada catatan yang memperlihatkan ia pernah didengar
(`absent-user-objects`); selama belum ada catatan sama sekali,
peringatannya menyarankan untuk mulai dengan mencatat percakapan pertama.

**Lebih dalam.** `docs/maintenance.md` (Communications, sessions, test
sessions and deploys), `specarch help asking` (mencatat apa yang
dilaporkan analis dari percakapan), `specarch help log`.

<!-- situation: requirement-detailed -->
## Kebutuhan yang dirinci

**Kapan terjadi.** Sebuah kebutuhan tadinya satu kalimat. Sekarang
entitas yang dibutuhkannya, operasi yang melaksanakannya, aturan yang
tersirat darinya dan tes yang memperlihatkan ia terpenuhi harus ditulis,
dan kalimat itu harus tetap terlacak ke masing-masing.

**Apa yang dijalankan.**

    specarch validate spec
    specarch derive --dry-run spec
    specarch derive --fill spec
    specarch try lateFee spec
    specarch document traceability --out docs spec
    specarch document testplan --out docs spec

Entitas, operasi, algoritme atau halaman menyebut `satisfies: [LIB-5]`;
tes menyebut `verifies: [LIB-5]`; validator memperingatkan
`requirement_unverified` pada kebutuhan yang tidak diverifikasi apa pun.
`specarch derive` menulis draf tes untuk setiap kasus yang tersirat dari
rancangan dan belum dicakup tes (kolom yang hilang, nilai dengan bentuk
salah, pemanggil tanpa izin) dan `--fill` menambahkan fixture, input dan
expect yang ditetapkan rancangan. Algoritme membawa contoh kerja, dan
`specarch try` mengevaluasi rumusnya pada masing-masing dan mencetak
setiap langkah. Dokumen traceability adalah matriks dari kebutuhan
pemangku kepentingan lewat kebutuhan sampai elemen, tes dan pemeriksaan;
rencana uji berisi kasus-kasusnya, golden dan red, terlacak ke kebutuhan.

**Contoh.** Kebutuhan yang belum punya tes, dan algoritme dengan empat
contoh:

    $ specarch validate examples/lending-desk
    examples/lending-desk/spec/requirements/requirements.yaml:53:3: warning: /requirements/LEND-4: requirement_unverified: no test, commissioning check or monitor verifies requirement LEND-4; add verifies: [LEND-4] to the test that shows it is met [requirement_unverified@requirements/requirements.yaml#/requirements/LEND-4]
    specarch: 1 input checked: 0 errors, 1 warning, 8 open questions
    $ specarch try lateFee examples/library-lending
    lateFee: 4 examples, 4 pass
    example 1 "returned on time": pass
      inputs: daysLate 0, dailyRate "0.50", replacementCost "25.00"
      decimal(daysLate, 0) = "0"
      decimal(daysLate, 0) * dailyRate = "0.00"
      min(decimal(daysLate, 0) * dailyRate, replacementCost) = "0.00"
      result "0.00", expected "0.00"
    ... 3 more examples

**Yang perlu diperhatikan.** Rincian yang datang dari keputusan membawa
`origin: decided` dan `decidedIn`; rincian yang disimpulkan dari bukti
membawa `origin: inferred` dan `why`. Contoh kerja adalah tes pertama
sebuah algoritme: ambil angkanya dari catatan nyata. Pada profil full,
subjek tanpa tes golden-nya adalah kesalahan, jadi tulis tes bersama
elemennya, atau biarkan `derive` mendrafnya. Tipe bersifat konkret dan
sadar lebar: `int64` yang dikirim sebagai angka JSON kehilangan digit di
atas 2^53 di JavaScript.

**Lebih dalam.** `docs/test-generation.md`, `docs/traceability.md`
(matriks SpecArch sendiri, dibangkitkan), `docs/conventions.md` (Types),
`specarch help try`, `specarch help validate`.

<!-- situation: draft-before-approval -->
## Membangkitkan draf sebelum penyetujuan untuk diperlihatkan ke pengguna

**Kapan terjadi.** Spesifikasi baru setengah ditulis dan pertanyaannya
masih terbuka, tetapi pengguna lebih memahami layar daripada dokumen, dan
makin cepat mereka melihatnya makin cepat mereka mengatakan apa yang
kurang.

**Apa yang dijalankan.**

    specarch document html --out docs spec
    specarch document techspec --out docs spec
    specarch generate ui --unapproved --out drafts/web spec
    specarch generate ui --unapproved --environment staging --out drafts/staging spec

Dokumen ditulis untuk spesifikasi yang belum lengkap, setiap masalah dan
setiap pertanyaan terbuka ditandai pada elemennya; `document html` adalah
seluruh spesifikasi sebagai satu halaman tanpa server, dengan setiap layar
yang dibangkitkan di dalamnya. `generate --unapproved` membuat kode dari
spesifikasi yang belum disetujui, setiap berkas ditandai sebagai draf.
Lingkungan yang entrinya menyebut `trace: true` menghasilkan build yang
memperlihatkan jejak: UI web menamai elemen setiap halaman, bagian,
kolom, lajur dan aksi, menggambar lapisan dengan elemen di bawah fokus,
versi dan digest build-nya dan sebuah keberatan untuk disalin sebagai
permintaan perubahan; server Go menjawab dengan header
`Specarch-Element`.

**Contoh.** Lingkungan staging perpustakaan memperlihatkan jejak, dan
build berjejak masuk ke folder tersendiri:

    $ specarch generate ui --dry-run --unapproved --environment staging examples/library-lending
    specarch generate: staging is an environment of examples/library-lending/spec whose entry says trace: true, and its build would go into examples/library-lending/web, the output folder the implementation file names; that folder holds the untraced build, the one generate --check compares, so a build that shows the trace goes under --out: give --out <folder> and run it again
    $ specarch generate sql --dry-run --unapproved examples/library-lending
    specarch generate: 0 files to write, 1 file already current; nothing written (--dry-run)

**Yang perlu diperhatikan.** Draf bukan untuk apa pun yang akan dipakai:
berkasnya mengatakan demikian di header-nya. Jejak menamai bagian dalam
sistem, jadi hanya lingkungan yang memintanya yang memperlihatkannya,
tidak pernah produksi. Setiap berkas yang dibangkitkan menyebut versi dan
digest spesifikasi, sehingga keberatan atas draf bisa menyebut build mana
yang dimaksud. Keberatan kembali sebagai permintaan perubahan
(`absent-user-objects`), tidak pernah sebagai suntingan pada berkas yang
dibangkitkan, yang ditimpa pembangkitan berikutnya.

**Lebih dalam.** `docs/acceptance.md` (1, Show a result early; 2, Trace a
result to the specification), `specarch help generate`.

<!-- situation: approving -->
## Menyetujui spesifikasi

**Kapan terjadi.** Pertanyaan must dan should sudah dijawab, dokumen
sudah ditulis dan orang yang memutuskan sudah membacanya. Kode yang akan
dipakai hanya boleh dibangkitkan dari spesifikasi yang disetujui orang
itu.

**Apa yang dijalankan.**

    specarch document techspec --out docs spec
    specarch document requirements --out docs spec
    specarch document problems --out docs spec
    specarch approve --dry-run --by librarian --session SES-1 spec
    specarch approve --by librarian --session SES-1 spec

Catatan penyetujuan, `records/approvals/<versi>.yaml`, menyimpan digest
setiap berkas, sehingga perubahan apa pun sesudahnya membatalkannya, dan
commit tempat ia dibuat bila setiap berkas sudah di-commit. `approve`
menolak spesifikasi dengan kesalahan, dengan pertanyaan must atau should
yang terbuka, atau yang dokumennya berbeda dari apa yang dikatakannya
sekarang, dan role yang belum pernah didengar: belum ada communication,
session, interview atau decision yang menyebutnya (rule `unheard`).
`--session` menyebut record session, test session atau communication
tempat role itu membaca dokumennya, dan record itu harus mencantumkan role
tersebut; tanpa `--session`, `approve` memberi peringatan lalu tetap
jalan. `document problems` menulis `problems.txt` dan `problems.sarif`,
setiap kesalahan, peringatan dan pertanyaan terbuka satu baris
masing-masing, untuk dibaca penyetuju lebih dulu.

**Contoh.**

    $ specarch approve --dry-run --by librarian --session SES-1 library
    would write library/records/approvals/0.1.0.yaml
    specarch approve: version 0.1.0 of library/spec can be approved by librarian; nothing written (--dry-run)

**Yang perlu diperhatikan.** Hanya kata orang itu sendiri yang menjalankan
`approve`: agen bertanya dan berhenti. Penyetujuan adalah atas persis apa
yang dibaca, seutuhnya: sebagian spesifikasi tidak bisa disetujui, dan
biaya menyetujui lagi ditekan oleh `diff` (`approve-again`). Folder dengan
perubahan yang belum di-commit mendapat penyetujuan tanpa commit, dengan
satu baris di standard error yang menyebut alasannya.

**Lebih dalam.** `docs/refinement.md` (Approval, The gate on code
generation), `specarch help gaps`, `specarch help approve`.

<!-- situation: building -->
## Membangun sistem dari spesifikasi

**Kapan terjadi.** Spesifikasi sudah disetujui, stack sudah dipilih, dan
migrasi, API, pengikatan server, layar, tes dan data pengembangan akan
ditulis darinya, bukan dengan tangan.

**Apa yang dijalankan.**

    specarch generate sql spec
    specarch generate seed spec
    specarch generate sqlc spec
    specarch generate scripts spec
    specarch generate openapi spec
    specarch generate go-dxlib spec
    specarch generate tests spec
    specarch generate ui spec
    specarch generate bpmn spec
    specarch generate deploy spec
    specarch idioms spec
    specarch idioms diff type-rendering spec
    specarch generate ui --estate estate/specarch-estate.yaml estate/reading-room

Setiap target adalah plug-in di PATH, `specarch-gen-<target>` atau
`specarch-gen-<target>-<stack>` untuk stack tersendiri:
`specarch-gen-sql` (migrasi untuk PostgreSQL, SQL Server, Oracle atau
MariaDB), `specarch-gen-seed` (baris pengembangan), `specarch-gen-sqlc`
(kueri), `specarch-gen-scripts` (skrip data berpengaman),
`specarch-gen-openapi`, `specarch-gen-go-dxlib` (pengikatan server Go),
`specarch-gen-tests-go`, `specarch-gen-tests-swift` dan
`specarch-gen-tests-dart` (tes dari contoh kerja dan tes rancangan),
`specarch-gen-ui` (JavaScript polos), `specarch-gen-ui-typescript`
(Next.js dan Carbon), `specarch-gen-ui-swiftui` (iPhone),
`specarch-gen-ui-vue-htm` (web tanpa langkah build), `specarch-gen-bpmn`
(setiap alur kerja sebagai BPMN dan SVG), `specarch-gen-register`
(`registers`) dan `specarch-gen-deploy` (setiap deployment sebagai berkas
Compose, manifest Kubernetes atau berkas cloud-init per host, runbook
release, rollback dan migrasi, serta skrip environment fresh yang
mem-build, memuat, menguji dan membongkar deployment Compose, setiap
secret dirujuk dan tidak pernah ditulis). Berkas implementasi di bawah `implementation/<stack>/`
menyebut stack, folder keluaran setiap target dan idiom yang
ditimpanya; `specarch idioms` mendaftarnya dan `idioms diff` mencetak
bagian-bagian idiom bawaan di samping penimpanya. `--estate` membawa
modul-modul suatu estate ke dalam jalannya, sehingga rujukan ke dalamnya
terselesaikan. `specarch help plugins` mendaftar plug-in yang ditemukan di
PATH dan apa yang ditulis dan tidak ditulis masing-masing.

**Contoh.** Implementasi Go perpustakaan memakai lima belas idiom bawaan:

    $ specarch idioms examples/library-lending
    examples/library-lending/spec/implementation/go/library-lending.go.specarch-implementation.yaml
      audit-fields 1.1.0: shipped
      authorization-check 1.0.0: shipped
      background-jobs 1.0.0: shipped
    ... 12 more idioms, then the JavaScript, Swift and TypeScript files

**Yang perlu diperhatikan.** Plug-in menulis apa yang dicakup targetnya,
menandai setiap entri yang tidak bisa ditulisnya pada tempatnya, dan
menyebut apa yang tidak pernah ditulisnya; sisanya ditulis dengan tangan,
di berkas yang tidak ditulis target, karena `generate` menulis setiap
berkas yang dijawab plug-in (sqlc dan scripts tidak pernah menimpa berkas
yang tidak mereka tulis). `sql` menolak perubahan yang tidak bisa
dibedakannya dari penulisan ulang, beserta apa yang harus dikerjakan
dengan tangan. Tidak ada yang khusus stack masuk ke rancangan: tempatnya
di berkas implementasi.

**Lebih dalam.** `docs/generators.md` (setiap target dan protokol
plug-in), `docs/idioms.md`, `docs/ui-design.md`, `docs/ui-nextjs-carbon.md`,
`docs/ui-swiftui.md`, `docs/ui-vue-htm.md`, `docs/estate.md`,
`docs/dxlib-lessons.md`.

<!-- situation: ci -->
## Menjaga spesifikasi dan kode tetap sama di CI

**Kapan terjadi.** Seseorang menyunting berkas yang dibangkitkan dengan
tangan, atau mengubah basis data tanpa mengubah spesifikasi, dan tak
seorang pun menyadari sampai pembangkitan berikutnya menimpanya atau rilis
berikutnya merusak klien.

**Apa yang dijalankan.**

    specarch validate spec
    specarch generate sql --check spec
    specarch document techspec --check --out docs spec
    specarch check --allowlist check-allowlist.txt --since HEAD spec routes.json permissions.json catalogue.json
    specarch skill --out .agents/skills --check

`generate --check` dan `document --check` membuat keluaran di memori dan
keluar dengan status 1 bila berkas di disk berbeda. `specarch check`
membandingkan spesifikasi dengan dump kode yang berjalan (rutenya,
izinnya, katalog basis datanya, berkas register) dan gagal pada perbedaan
yang tidak disebut allowlist. `skill --check` gagal bila skill agen di
repositori sudah basi.

**Contoh.**

    $ specarch generate sql --check --unapproved examples/library-lending
    specarch generate --check: 1 file checked, 0 files differ
    $ specarch check --allowlist service/check-allowlist.txt --since HEAD project service/routes.json service/permissions.json service/catalogue.json
    specarch check: 3 dumps compared: 0 differences, 3 allowed by the allowlist, 2 reports, 0 allowlist lines that fail
    project/specarch.yaml:27:7: warning: /entities/Copy/constraints/copies_title_given: check_expression_differs: the check copies_title_given of copies reads (title <> '') in the specification's table and (((title)::text <> ''::text)) in the database; the database rewrites a check's text, so this is a report [check_expression_differs copies.copies_title_given]
    service/catalogue.json:203:17: warning: /tables/2/indexes/1: index_not_listed: the index copies_title_lower of copies, CREATE INDEX copies_title_lower ON public.copies USING btree (lower((title)::text)), is in the database and the mapping of entity Copy does not list it; an index is physical design, so this is a report: list it under the mapping's indexes when it is plain [index_not_listed copies.copies_title_lower]

**Yang perlu diperhatikan.** Berkas tanpa header pembangkitan dibaca
sebagai dibuat dengan tangan: `--check` mengatakan apakah ia cocok dengan
keluaran kecuali header-nya. Laporan adalah perbedaan yang bukan
kegagalan (basis data menulis ulang teks sebuah check; indeks adalah
rancangan fisik); perbedaan adalah kegagalan. Jalankan pemeriksaan ini
sebelum setiap commit dan di CI, dan validator Swift di samping yang Go
bila keduanya dibangun: keduanya mencetak keluaran yang sama.

**Lebih dalam.** `docs/sync-gates.md`, `scripts/landing-check.sh` (setiap
pemeriksaan yang dijalankan repositori ini), `specarch help check`.

<!-- situation: test-scripts -->
## Memakai test script yang sudah dimiliki tim

**Kapan terjadi.** Tim, atau customer, sudah punya test case: file
feature Gherkin, atau test sheet yang tiap row-nya berisi precondition,
step, test data dan expected result. Ada row yang belum punya expected
result, ada scenario yang berhenti sebelum Then, dan semuanya belum masuk
ke spesifikasi.

**Apa yang dijalankan.** Commit dulu script-nya, lalu baca ke dalam
sebuah tree dan merge dengan tree yang lain:

    specarch extract tests --out trees/tests scripts/notes.feature scripts/regression.xlsx
    specarch merge --out spec trees/code trees/openapi trees/tests
    specarch gaps --next spec

`extract tests` menulis satu test per scenario, example atau row, dengan
level system, red kalau script-nya bilang negative atau expected
result-nya menyebut penolakan atau error, `origin: inferred`, dan cites ke
baris atau row asalnya. Setiap test dapat satu must question untuk
subject yang di-test, dengan menyebut request yang disebut script
(`POST /notes`), dan satu must question untuk setiap given, step atau
expected result yang belum ditulis di script. `merge` menyimpan test ini
utuh bersama question-nya. Jawab satu per satu dengan `specarch decide`,
tulis apa yang diputuskan ke test-nya, dan test itu masuk ke test package.

**Contoh.** Scenario tanpa Then dan row sheet yang hanya berisi ID dan
title (`spec/tests/extract-tests-incomplete-scripts`):

    read notes.feature: 4 tests
    read sheet.csv: 2 tests
    wrote 6 tests and 10 questions: one per subject a test exercises, which a script cannot name in the specification's words, 1 for a test with no given, 1 with no step and 2 with no expected result

**Yang perlu diperhatikan.** Test tidak pernah diisi bagian yang tidak
ditulis di script-nya: scenario tanpa expected result tetap golden sampai
question-nya dijawab. Question untuk subject mem-block seluruh test,
jadi test itu ditahan dari package sampai dijawab. Hanya keyword Gherkin
bahasa Inggris yang dibaca; file feature dengan header `# language:`
dibaca sebagai teks tanpa step.

**Lebih dalam.** `docs/extraction.md` (Test scripts), `specarch help
extract`, ADR-235.

<!-- situation: test-package -->
## Menjalankan test di environment milik customer

**Kapan terjadi.** Customer yang menjalankan environment acceptance,
tidak memegang code maupun test harness project, dan ingin tahu path mana
dari spesifikasi yang jalan di sana: golden path, red path, dan path yang
belum ter-cover test.

**Apa yang dijalankan.** Buat package untuk environment itu, serahkan,
dan biarkan yang menjalankan environment-nya me-run dengan credential
mereka sendiri:

    specarch generate tests --package acceptance --out package spec
    specarch run --callers callers.yaml --setup ./put-records --results records/evidence/TR-4-junit.xml package
    specarch log test-run --results records/evidence/TR-4-junit.xml --environment acceptance --at 2026-11-04T10:00:00+07:00 --by customer-it spec

Package-nya, `specarch-test-package.yaml`, berisi semua test dan semua
derived case yang belum ter-cover test, masing-masing sebagai request ke
server di environment itu beserta apa yang harus ada di jawabannya, atau
beserta alasan kenapa tidak bisa dijalankan dari luar. `callers.yaml`
memetakan setiap role ke header untuk sign in; setup program menyiapkan
record yang dibutuhkan sebuah case. Keduanya tidak masuk ke package
maupun ke hasil. JUnit XML-nya di-map balik lewat `log test-run`, derived
case ke elemen subject-nya.

**Contoh.** Package untuk catatan yang disimpan
(`spec/tests/generate-tests-package`), dan run-nya terhadap server yang
tidak menjawab (`spec/tests/run-package-no-answer`):

    specarch generate: held back keep-note-seen, which Q-1 holds up; it joins the package once answered
    error: list-notices-succeeds: GET http://127.0.0.1:1/notices got no answer: Get "http://127.0.0.1:1/notices": dial tcp 127.0.0.1:1: connect: connection refused

**Yang perlu diperhatikan.** Case dengan role yang tidak ada di file
callers, atau yang butuh record tanpa setup program, di-skip dengan
alasannya, bukan dianggap gagal. Test yang masih ditahan open question
tidak dimasukkan sampai question-nya dijawab. State setelahnya dan
message yang di-emit tidak dicek dari luar; suite milik project yang
mengeceknya. Kalau environment-nya punya beberapa server, pilih satu
dengan `--server` atau berikan `--base-url`.

**Lebih dalam.** `docs/test-generation.md` (The test package),
`specarch help run`, ADR-233.

<!-- situation: test-results -->
## Menyimpan hasil test setiap build

**Kapan terjadi.** CI menjalankan test di setiap build lalu hasilnya
dibuang. Saat user acceptance test, customer bertanya requirement mana
yang sudah terbukti terpenuhi di build yang sedang mereka lihat, dan
bagian sistem mana yang terus gagal dari satu build ke build berikutnya.

**Apa yang dijalankan.** Taruh berkas JUnit XML dari test runner di bawah
`records/`, lalu:

    specarch log test-run --results records/evidence/TR-1-junit.xml --environment staging --version 0.2.0 --build "library-lending 0.2.0 at 5d3e8b1" --at 2026-10-14T09:30:00Z --by library-it spec
    specarch document testruns spec

`log test-run` menulis `records/test-runs/TR-<n>.yaml`: environment,
version, build dan waktu run, siapa yang menjalankan, berkas hasil sebagai
evidence beserta SHA-256-nya, dan satu result per testcase yang di-map ke
test di spesifikasi, sehingga ke elemen yang di-test dan requirement yang
di-verify oleh test itu. `--open-defect <test>` bersama `--severity` dan
`--phase` membuka defect untuk failure yang Anda nilai memang defect.
`document testruns` menampilkan run-run tersebut dan jumlah failure per
elemen dan per requirement, satu kolom per run; `timeline` menempatkan
setiap run di waktunya.

**Contoh.** Run pertama di contoh library lending, di
`examples/library-lending`:

    $ specarch log test-run --results records/evidence/TR-1-junit.xml --environment staging --version 0.2.0 --build "library-lending 0.2.0 at 5d3e8b1" --at 2026-10-14T09:30:00Z --by library-it spec
    records/test-runs/TR-1.yaml
    specarch log test-run: TR-1 written to records/test-runs/TR-1.yaml, 5 results: 4 passed, 1 failed, 0 errored, 0 skipped, 0 unmapped

dan di `docs/testruns.md`, setelah run kedua:

    | Requirement | TR-1 | TR-2 | Open defects |
    |---|---|---|---|
    | LIB-3 | 0 | 0 | none |
    | LIB-5 | 1 | 0 | none |

Testcase yang test-nya belum ada di spesifikasi tetap disimpan, tidak
dibuang (`spec/tests/log-test-run-writes-record`):

    unmapped: TestRenewal: no test of the specification is named so, nor Test or test and its name in Pascal case, nor does the name begin with a test's name and a colon; nor is it a test the generators write for an algorithm's or a call mapping's worked examples, nor a case no test covers as a test package names it

**Yang perlu diperhatikan.** Testcase di-map lewat nama yang ditulis oleh
generator test (`TestReturnLate`, `testReturnLate()`, `return-late: ...`),
dan test hasil generate untuk worked example sebuah algorithm atau call
mapping (`TestAlgorithmLateFee`, `algorithm lateFee: ...`) menyebut
element-nya di `element`; runner yang menamai test dengan cara lain akan
menyisakan result yang tidak ter-map, dan `validate` memberi warning
`test_unmapped` di setiap result itu sampai `test` atau `element` ditulis
di result-nya. Timestamp di berkas tidak
dibaca karena tidak punya offset: isi `--at`. Failure tidak otomatis
membuka defect, karena host yang tidak stabil juga bisa membuat test
gagal. Simpan berkas hasil setelah dicatat: rule `evidence` gagal kalau
berkas itu berubah.

**Lebih dalam.** `docs/maintenance.md` (Test runs), `specarch help log test-run`,
ADR-230.

<!-- situation: comparing-implementations -->
## Memilih di antara dua implementasi

**Kapan terjadi.** Tim membangun dua implementasi dari satu design, dua
stack atau dua library untuk satu stack, lalu harus memilih salah satu.
Evidence-nya tersebar di log CI, laporan scan, dan ingatan orang tentang
masing-masing build, sehingga decision yang dipilih belum men-cite satu
pun dari itu.

**Apa yang dijalankan.** Simpan input setiap build di bawah `records/`:
test run-nya (`specarch log test-run`) atau berkas JUnit XML-nya, JSON
grype atau osv-scanner dari software bill of materials-nya, dan berkas
measurements, JSON berisi `buildSeconds`, `linesOfCode`, `deploySteps`
dan, per requirement, `latency` dan `throughput` sebagai
`{value, unit, how}`. Lalu:

    specarch compare --test-run lending.go=TR-3 --results lending.swift=records/evidence/swift-junit.xml --scan lending.go=records/evidence/go-grype.json --scan lending.swift=records/evidence/swift-osv.json --measurements lending.go=records/evidence/go-measurements.json --measurements lending.swift=records/evidence/swift-measurements.json --out records/comparisons/lending spec spec/implementation/go/lending.go.specarch-implementation.yaml spec/implementation/swift/lending.swift.specarch-implementation.yaml

Setiap option menyebut implementasi dengan nama berkasnya tanpa
`.specarch-implementation.yaml`. `compare` menulis satu tabel, satu kolom
per implementasi: derived case yang passed, hasil test, temuan scan per
severity, library beserta licence-nya, waktu build, jumlah baris kode dan
langkah deploy; lalu satu baris per derived case, per requirement yang
diukur di samping statement-nya, per library dan per temuan; input beserta
SHA-256-nya; dan gap-nya. Dengan `--out` perintah ini menulis
`comparison.md` dan `comparison.json`, lalu mencetak entri `sources` yang
dipakai decision untuk men-cite-nya.

**Contoh.** `spec/tests/compare-two-implementations`:

    | Summary | lending.go | lending.swift |
    |---|---|---|
    | Derived cases passed | 0 of 2 | 1 of 2 |
    | Build time | 74 s | gap: not measured |

dan di standard output:

    to cite the comparison from a decision, declare it under sources in specarch.yaml:
      comparison-lending: {kind: "document", title: "Comparison of lending.go and lending.swift", edition: "sha256:...", url: "../records/comparisons/lending/comparison.md"}

**Yang perlu diperhatikan.** Hasil yang belum ada ditulis sebagai gap,
bukan nol: di sel-nya tertulis `gap`, dan gap itu ada di daftar beserta
option atau key yang bisa mengisinya, jadi baca bagian Gaps dulu sebelum
summary. Latency dan throughput ditaruh di samping kalimat requirement-nya,
tidak dinilai otomatis. Test run tidak menyebut implementasi, jadi Anda
yang menentukan run mana milik implementasi mana setiap kali menjalankan.
Input yang diberikan tapi salah ditolak dengan exit 1, tidak ditampilkan
sebagai gap. Edition-nya adalah SHA-256 dari `comparison.json`: jalankan
`compare` lagi kalau ada input yang berubah, lalu update entri-nya.

**Lebih dalam.** `specarch help compare`, ADR-232.

<!-- situation: small-change -->
## Perubahan kecil

**Kapan terjadi.** Sebuah kolom mendapat panjang, sebuah daftar mendapat
lajur, sebuah pesan ditulis ulang. Sudah disetujui, sudah dipakai, dan
perubahannya pekerjaan satu pagi.

**Apa yang dijalankan.**

    specarch validate spec
    specarch diff spec-at-approval spec
    specarch generate sql --check spec
    specarch document changes --out docs spec

Perubahan adalah catatan lebih dulu, `records/changes/CR-<n>.yaml`: siapa
yang mengajukannya, kapan, fasenya (development, acceptance atau
production), tipe dan urgensinya, `affects` menurut tahap dan `impact`
(patch, minor atau major, menurut apa yang dilakukannya pada antarmuka
publik). Lalu spesifikasi berubah, `validate` memeriksanya, `generate
--check` memperlihatkan berkas bangkitan mana yang kini basi, dan `diff`
mendaftar apa yang berubah di antara dua versi spesifikasi dan memeriksa
rilis di antaranya.

**Contoh.** Tiga perubahan di antara dua versi sebuah kasus:

    $ specarch diff old/spec new/spec
    minor changed #/entities/Item: properties/weight is added
    minor added #/paths/~1items~1{itemId}/get
    patch changed #/requirements/CASE-1
    specarch diff: 3 elements changed, 0 errors, 0 warnings

**Yang perlu diperhatikan.** Penyetujuan tidak lagi berlaku begitu sebuah
berkas berubah: perubahan dibaca dan disetujui lagi (`approve-again`),
yang dibuat murah oleh `diff`. Sebelum 1.0.0 perubahan yang merusak
menaikkan angka minor. Perubahan membawa alasannya; spesifikasi tidak
membawa log perubahan di dalam berkas: keputusan mengatakan mengapa sebuah
elemen seperti adanya, dan catatan serta git mengatakan apa yang berubah
kapan.

**Lebih dalam.** `docs/maintenance.md` (Change requests, Releases and
versions, The diff verb), `docs/changes.md` (register SpecArch sendiri,
dibangkitkan).

<!-- situation: large-change-late -->
## Perubahan besar di akhir siklus hidup

**Kapan terjadi.** Beberapa minggu sebelum go-live, pelanggan meminta
sesuatu yang menjangkau selusin elemen: jenis anggota kedua, langkah
persetujuan baru di setiap alur kerja, sebuah kolom yang diperlihatkan
setiap layar. Semua orang ingin tahu apa yang disentuhnya sebelum ada yang
mengatakan ya.

**Apa yang dijalankan.**

    specarch document changes --out docs spec
    specarch decision rests ADR-001 spec
    specarch gaps spec
    specarch decide Q-9 --by library-board --answer "<pilihan yang diambil>" --record COM-7 spec
    specarch diff spec-at-approval spec
    specarch document releases --out docs spec

Permintaan perubahan ditulis dan dianalisis: `affects` menyebut apa yang
ditambah, diubah dan dihapusnya. Di bawah setiap perubahan terbuka,
dokumen changes memperlihatkan apa yang dijangkaunya di spesifikasi
sebagaimana adanya sekarang: elemen yang disebut pointer-nya, tes
tentangnya, halaman, operasi, perintah dan saluran yang membaca entitas di
antaranya, target kode yang mengambil bagian yang disentuhnya, dokumen
yang memperlihatkannya, dan penyetujuan yang dibatalkannya. `decision
rests` mencetak keputusan-keputusan yang bersandar pada keputusan yang
akan dibatalkan perubahan itu. Analisis berakhir pada keputusan yang
menyebut perubahan itu di bawah `answers`, pada pertanyaan terbuka yang
disebut perubahan itu, atau pada penolakan di catatannya.

**Contoh.** Apa yang bersandar pada keputusan pertama perpustakaan, dan
apa yang dijangkau perubahan terbuka:

    $ specarch decision rests ADR-001 examples/library-lending
    2 decisions rest on ADR-001, nearest first
    ADR-001: Late fees are a flat daily rate capped at the replacement cost (accepted, 2026-10-07)
    ADR-003: The late fee is computed once, at return, and stored on the loan (accepted, 2026-10-25; rests on ADR-001, ADR-002; from idea IDEA-1)
    ADR-004: A waiver is a request of its own that names the amount and the reason, and the fee stands until the request is approved (accepted, 2026-10-25; rests on ADR-003)

Dokumen changes perpustakaan mendaftar CR-3 di antara item terbuka beserta
fase, tipe, urgensi, dampak dan sebabnya, dan di bawahnya apa yang
dijangkau perubahan itu, sebelum apa pun berubah.

**Yang perlu diperhatikan.** Perubahan dengan `impact: major` setelah
1.0.0 menaikkan versi major, dan kenaikan versi rilis harus setidaknya
sebesar perubahan terbesarnya: `diff` memeriksanya. Persetujuan tanpa
dampak tertulis adalah cara biasa permintaan kecil merusak antarmuka:
analysed adalah status tersendiri. Elemen yang diubah sebuah keputusan
membawa `origin: decided` dan `decidedIn`, dan keputusan itu mengatakan
mengapa.

**Lebih dalam.** `docs/acceptance.md` (4, What it reaches, before it
changes), `docs/maintenance.md`, `docs/decision-tracking-2026-10-25.md`.

<!-- situation: absent-user-objects -->
## Pengguna yang absen berkeberatan pada uji terima

**Kapan terjadi.** Staf meja tidak ada di rapat kebutuhan; dewan dan
orang TI ada. Pada uji terima pengguna, meja melihat layar untuk pertama
kali dan mengatakan aksi yang paling sering dipakainya ada di bawah
lipatan.

**Apa yang dijalankan.**

    specarch validate spec
    specarch log spec --recorded-by librarian --direction in --from librarian --to library-it --at 2026-10-21T10:40:00Z --channel meeting --type objection --words "..." --form summary --session TS-1 --element "#/pages/loans-list" --change CR-3
    specarch document changes --out docs spec
    specarch decide Q-12 --by library-board --answer "<pilihan yang diambil>" --record COM-5 spec
    specarch decision why ADR-005 spec

Sebelum uji, `validate` memperingatkan `unheard` pada pemangku
kepentingan yang diandalkan kebutuhan yang diterima atau halaman yang
disetujui, yang tidak ada komunikasi, sesi, wawancara atau keputusan
memperlihatkan ia pernah didengar. Pada uji, keberatan dicatat sebagai
objection, lalu diajukan sebagai permintaan perubahan dengan `phase:
acceptance`, build yang dimaksud (versi, lingkungan, digest, build),
`seenAt` disalin dari jejak build, `observed` dan `expected` dengan
kata-kata penguji, dan catatannya ditautkan. Analisisnya menulis `cause`:
`omitted` (spesifikasi melewatkannya), `wrong` (spesifikasi
mengatakannya, dan itu bukan yang dimaksud), `changed` (kebutuhannya
bergeser) atau `unheard` (peran yang memakainya absen atau tidak pernah
ditanya), dan berakhir pada keputusan yang menyebut perubahan itu,
pertanyaan, atau penolakan. `decision why` mencetak rantai yang menjadi
sandaran sebuah keputusan.

**Contoh.** Spesifikasi yang disimpan dengan catatan, dua peran tidak
pernah didengar:

    $ specarch validate project
    project/specarch.yaml:5:9: warning: /stakeholders/desk: unheard: desk was never heard: no communication record names it, no session lists it present, no interview lists it and no decision is by it, while requirement CASE-1 and page members-list rely on it; hear it before acceptance, and record what it said with specarch log [unheard@specarch.yaml#/stakeholders/desk]
    project/specarch.yaml:7:10: warning: /stakeholders/board: unheard: board was never heard: no communication record names it, no session lists it present, no interview lists it and no decision is by it, while requirement CASE-1 relies on it; hear it before acceptance, and record what it said with specarch log [unheard@specarch.yaml#/stakeholders/board]

Pada contoh perpustakaan rantainya ada di timeline: SES-1, rapat
perpanjangan, mendaftar `library-it` diundang dan absen; TS-1 adalah uji
terima 0.2.0 di staging; COM-4 adalah keberatan meja di sana; CR-3 adalah
permintaan perubahan terhadap build 0.2.0 di staging, `seenAt:
["#/pages/loans-list"]`, dan ADR-005 adalah keputusan yang menyetujuinya:

    $ specarch decision why ADR-005 examples/library-lending
    ADR-005 rests on no decision
    ADR-005: The actions of the loans list stand in the order the desk uses them, Renew first (accepted, 2026-10-28)

**Yang perlu diperhatikan.** Unheard dipisahkan dari omitted karena
obatnya berbeda: bukan pena yang lebih baik, melainkan kursi di meja untuk
peran yang disebut peringatan itu, sebelum uji. Keberatan diterapkan pada
spesifikasi, tidak pernah pada kode yang dibangkitkan, yang ditimpa
pembangkitan berikutnya; jejaklah yang memungkinkan keberatan menyebut
elemennya. Cacat diukur terhadap spesifikasi; perubahan terhadap apa yang
dibutuhkan pengguna, jadi keberatan yang sudah dijawab spesifikasi adalah
cacat.

**Lebih dalam.** `docs/acceptance.md` (3, A remark is a change request;
tabel sebab), `docs/maintenance.md` (Change requests), `docs/timeline.md`
dan `docs/changes.md` pada contoh perpustakaan.

<!-- situation: approve-again -->
## Menyetujui lagi, dengan diff

**Kapan terjadi.** Spesifikasi sudah disetujui, lalu berubah: permintaan
perubahan dilaksanakan, pertanyaan dijawab, format ditingkatkan. Penyetuju
tidak akan membaca tiga ratus halaman lagi demi satu lajur.

**Apa yang dijalankan.**

    specarch diff spec-at-approval spec
    specarch decision why ADR-005 spec
    specarch approve --by librarian --session SES-2 spec

Catatan penyetujuan menyimpan commit tempat ia dibuat. `diff` antara
commit itu dan pohon kerja mendaftar setiap elemen yang berubah beserta
permintaan perubahan atau cacat yang mencakupnya, pemeriksaan versinya
dilewati bila kedua sisi membawa `info.version` yang sama. Penyetuju
membaca daftarnya, bukan spesifikasinya, dan menyetujui lagi, seutuhnya.

**Contoh.** Satu pra-rilis dibandingkan dengan dirinya sendiri, satu
nilai enum ditambahkan:

    $ specarch diff old/spec new/spec
    minor changed #/enums/Colour: gains the value blue

**Yang perlu diperhatikan.** Penyetujuan tetap utuh: digest-nya meliputi
setiap berkas, karena penyetujuan adalah atas persis apa yang dibaca;
sebagian spesifikasi tidak disetujui. Elemen yang berubah tanpa
permintaan perubahan atau cacat yang mencakupnya adalah kesalahan `diff`:
ajukan catatannya lebih dulu. Catatan rilis berstatus planned harus ada
untuk versi yang sedang dibangun.

**Lebih dalam.** `docs/acceptance.md` (8, What the owner decided),
`docs/maintenance.md` (The diff verb), `specarch help diff`,
`specarch help approve`.

<!-- situation: release -->
## Rilis dan catatan rilis

**Kapan terjadi.** Perubahan untuk 0.2.0 sudah dilaksanakan dan diuji,
dan orang yang menjalankan sistem ingin tahu apa isinya, cara
memasangnya, cara mengembalikannya dan siapa yang menandatanganinya.

**Apa yang dijalankan.**

    specarch diff spec-0.1.0 spec
    specarch document releases --out docs spec
    specarch document deployment --out docs spec
    specarch document commissioning --out docs spec
    specarch document handover --out docs spec
    specarch document manual --out docs spec
    specarch document timeline --out docs spec
    specarch document progress --out docs spec

Selagi rilis dibangun, `info.version` membawa tag pra-rilis
(`0.2.0-dev`) dan catatan rilis berstatus planned ada untuknya; pada
rilis, tag itu dilepas dalam commit yang sama yang mengubah catatan
menjadi released, menyebut permintaan perubahan dan cacat yang
`includes`-nya. `diff` antara dua spesifikasi yang ditandai memeriksa
kenaikan versi terhadap perubahan terbesar dan bahwa setiap elemen yang
berubah tercakup. Catatan rilis mengelompokkan perubahan setiap rilis
sebagai added, changed, removed dan fixed, dengan di mana masing-masing
diajukan dan sebabnya, dan menghitung perubahan menurut sebab; prosedur
commissioning adalah lembar tanda tangan; dokumen handover untuk para
insinyur pelanggan yang menjalankan lingkungan dari sumber; manual
pengguna untuk orang yang memakai sistem. Deploy adalah catatan,
`DEP-<n>`, satu pemasangan pada satu lingkungan pada satu waktu.
Laporan progress menghitung apa yang ada di spesifikasi dan catatannya:
elemen per stage, open question per prioritas dan role yang memutuskan,
role yang unheard, apakah daftar stakeholder sudah dikonfirmasi lengkap,
jam yang dicatat sebuah change request di `effort` (`estimatedHours`,
`spentHours`) dengan yang belum mencatat disebut sebagai missing, dan
burn-up elemen yang approved dan generated per rilis. Tidak ada angka yang
ditebak.

**Contoh.** Catatan rilis perpustakaan:

    $ specarch document releases --out docs examples/library-lending
    specarch document: 1 file written, 0 files already current
    $ sed -n 7,19p docs/releases.md
    ## 0.2.0, planned

    ### Added

    - CR-1, Let a member renew a loan once: `#/tests/renew-a-loan`; raised in production by member; cause omitted (the specification left it out).

    ### Changed

    - CR-1, Let a member renew a loan once: LIB-4, `#/entities/Loan`; raised in production by member; cause omitted (the specification left it out).

    ### Fixed

    - DEF-1, The late fee is not capped at the replacement cost: violated LIB-5; raised in production by librarian.

**Yang perlu diperhatikan.** Rilis dengan banyak perubahan bersebab
omitted, wrong atau unheard mengatakan pekerjaan kebutuhan menyisakan
celah; yang banyak changed hanya mengatakan kebutuhannya bergeser. Jalannya
commissioning yang menerima rilis dan deploy yang memasangnya adalah
catatan, sehingga timeline memperlihatkan urutannya: commissioning lolos,
lalu di-deploy. Antarmuka publik menurut SemVer adalah path, perintah,
saluran, rute dan setelan; klien yang ditulis untuk versi sebelumnya tidak
boleh rusak pada kenaikan minor.

**Lebih dalam.** `docs/maintenance.md` (Releases and versions),
`docs/releases.md` (catatan SpecArch sendiri, dibangkitkan),
`docs/commissioning.md`, `docs/deployment.md`.

<!-- situation: formats-move -->
## Rilis SpecArch memindahkan format

**Kapan terjadi.** Rilis baru program membaca spesifikasi yang ditulis
untuk meta-model yang lebih lama dan memperingatkan pada baris versinya.
Enam format masing-masing membawa versi pada baris pertama berkasnya:
meta-model (`specarch`), berkas implementasi (`specarchImplementation`),
catatan (`specarchRecord`), idiom (`specarchIdiom`), estate
(`specarchEstate`) dan kosakata (`specarchVocabulary`).

**Apa yang dijalankan.**

    specarch version
    specarch upgrade --dry-run spec
    specarch upgrade spec
    specarch validate spec
    specarch skill --out .agents/skills

`upgrade` hanya menulis ulang baris versi, petunjuk editor dan kunci yang
diganti namanya oleh versi baru, di tempat, mempertahankan komentar,
urutan dan tanda kutip; apa yang tidak bisa dikerjakannya secara mekanis
didaftarnya sebagai langkah yang diserahkan kepada penulis dan keluar
dengan status 1. Jalannya yang kedua tidak mengubah apa pun.

**Contoh.**

    $ specarch version
    specarch 1.5.0
    specifications: meta-model 1.0.0 and any later 1.0 patch, and 0.3, 0.2 and 0.1 with a warning
    implementation files, records, idioms and estates: 1.0.0 and any later 1.0 patch, and 0.1 with a warning
    vocabularies: 1.0.0 and any later 1.0 patch, and 0.1
    $ specarch upgrade --dry-run project
    project/deployment/handover.yaml:7:3: "source" -> "sourceDelivery"
    project/design/roles.yaml:20:5: rows -> rowPolicies
    project/specarch.yaml:3:11: '0.3' -> '1.0.0'
    ... 7 more changes to version lines and editor hints
    specarch upgrade: 10 changes to make in 6 files, 0 steps left to the author

**Yang perlu diperhatikan.** Menulis sebuah berkas mengubah spesifikasi,
jadi penyetujuan tidak lagi berlaku: setujui lagi setelah membaca dokumen
(`approve-again`). Pakukan program: catat versi modul dan checksum yang
dicetak `go version -m` di instruksi agen repositori. Tulis skill agen
lagi setelah peningkatan, sehingga tidak ada rujukan yang menyebut
perintah yang sudah tidak ada di rilis itu.

**Lebih dalam.** `docs/conventions.md` (Meta-model versions),
`specarch help upgrade`, `specarch help version`, catatan rilis setiap
rilis SpecArch.

<!-- situation: agents -->
## Agen yang mengemudikan SpecArch

**Kapan terjadi.** Agen di dalam chat, atau di pekerjaan CI, yang
mengerjakannya: ia menulis spesifikasi dari sumber, mengajukan
pertanyaan, mencatat jawaban dan membangkitkan kode, dan pemilik yang
memutuskan.

**Apa yang dijalankan.**

    specarch guide --agent
    specarch capabilities --json
    specarch help situations
    specarch model spec
    specarch validate --json spec
    specarch mcp
    specarch skill --out .agents/skills

`guide --agent` mencetak urutan kerja, cara membaca masalah, kapan
berhenti dan bertanya kepada pemilik, batasan dan konvensinya;
`AGENTS.md` di repositori ini memuat teks yang sama. `capabilities --json`
mendaftar setiap perintah sebagai tool dengan JSON Schema untuk
masukannya, keluarannya dan kode keluarnya, dan plug-in di PATH. `--json`
pada perintah mana pun mencetak satu objek; `--dry-run` pada perintah mana
pun yang menulis mendaftar apa yang akan ditulisnya. `model` mencetak
spesifikasi sebagai satu dokumen JSON untuk program. `mcp` melayani setiap
perintah sebagai tool dari Model Context Protocol lewat standard input dan
output; `skill` menulis skill agen beserta panduannya dan satu rujukan per
perintah. `specarch help agents` menjelaskan kedua jalan masuk itu.

**Contoh.**

    $ specarch skill --out skills --check
    specarch skill --check: 28 files checked, 0 files differ
    $ specarch model examples/library-lending | head -c 120
    {"estates":[],"implementations":[],"program":"1.5.0","specarchModel":"1.0.0","specifications":[{"document":{"accessibility":

**Yang perlu diperhatikan.** Program tidak bertanya dan tidak meminta apa
pun: langkah yang membutuhkan orang adalah perintah yang dijalankan orang
itu (`decide`, `approve`). Agen berhenti dan bertanya pada setiap
pertanyaan must atau should yang terbuka, pada pilihan apa pun di antara
dua sumber yang bertentangan, sebelum menyetujui, sebelum membangkitkan
draf untuk apa pun yang akan dipakai, dan sebelum menimpa berkas yang
dibuat dengan tangan. Ia mencatat siapa mengatakan apa dengan `specarch
log` dan tidak menebak siapa maupun kapan. Server MCP yang mendaftar
tool-nya tanpa sesi, revisi tanpa-status protokol itu, akan datang; klien
yang bisa keduanya kembali ke jabat tangan untuk sekarang.

**Lebih dalam.** `AGENTS.md`, `docs/from-sources.md` (5, The agent's
instructions), `specarch help agents`, `specarch help conventions`,
`specarch help mcp`, `specarch help skill`.
