# myc (My Commander)

Nowoczesny, szybki, dwupanelowy menedżer plików oferujący dwa zintegrowane tryby działania:
- **Tryb Desktop (GUI):** Klasyczny interfejs okienkowy z paskami wyboru dysków `[-c-]` `[-d-]` / `[/]`, kartami katalogów (tabs), tabelą kolumn z sortowaniem, zaznaczaniem plików na czerwono, paskiem narzędzi oraz dolnymi klawiszami funkcyjnymi F3–F8.
- **Tryb Terminal (CLI / TUI):** Klasyczny dwupanelowy menedżer plików w terminalu oparty o bibliotekę `tcell/v2`, z menu górnym (`F9`) oraz pełną obsługą skrótów klawiaturowych.

Napisany w czystym **Go 1.24**, ze wsparciem dla architektury 32-bitowej oraz 64-bitowej na systemach **Linux** i **Windows** bez zewnętrznych zależności CGo.

---

## 🚀 Główne możliwości

### 🗂️ Interfejs dwupanelowy
- **Dwa niezależne panele plików:** Płynna nawigacja, przełączanie aktywnego panelu klawiszem `Tab`.
- **Układ poziomy lub pionowy:** Przełączanie orientacji paneli w locie (`F2` lub `Ctrl+U`).
- **Górne menu ekranowe (`F9`):** Klasyczny pasek menu na samej górze ekranu (**Lewy**, **Plik**, **Polecenie**, **Opcje**, **Prawy**) z rozwijanymi listami komend (dropdown), nawigacją strzałkami i skrótami literowymi.
- **Pasek funkcyjny F1–F10:** Szybki dostęp do kluczowych akcji z poziomu dolnej belki.
- **Wiersz poleceń:** Wbudowany wiersz poleceń na dole ekranu z obsługą wykonywania poleceń w bieżącym katalogu.
- **Szybkie zaznaczanie wieloznacznikowe:**
  - `+` – zaznaczanie grupy plików wg wzorca (np. `*.go`, `*.txt`),
  - `-` – odznaczanie grupy plików wg wzorca,
  - `*` – odwrócenie zaznaczenia,
  - `Spacja` / `Insert` – zaznaczenie/odznaczenie pojedynczego pliku i przejście w dół,
  - `Ctrl+A` – zaznaczenie wszystkich plików.

### 📦 Wirtualny system plików (VFS) i obsługa archiwów
- Przeglądanie archiwów jak zwykłych katalogów:
  - **ZIP** (`.zip`, `.jar`)
  - **TAR** (`.tar`, `.tar.gz`, `.tgz`, `.tar.bz2`)
- Płynne wchodzenie do archiwum klawiszem `Enter` i wychodzenie przez katalog nadrzędny `..`.
- Możliwość bezpośredniego podglądu plików znajdujących się wewnątrz archiwum bez ręcznego rozpakowywania.

### 🛠️ Zaawansowane operacje na plikach
- **Kopiowanie, przenoszenie i usuwanie (`F5`, `F6`, `F8`):** Rekurencyjna obsługa drzew katalogów z dialogami potwierdzenia i wskaźnikiem postępu.
- **Narzędzie masowej zmiany nazw (Multi-Rename):** Zmiana prefiksów, sufiksów, rozszerzeń oraz wyszukiwanie i zamiana tekstu z numeracją porządkową (`%03d`).
- **Porównywanie plików wg zawartości (Diff):** Binarne oraz tekstowe porównywanie dwóch plików z raportowaniem przesunięć bajtowych lub różnic linia po linii.
- **Wyszukiwanie duplikatów plików:** Szybkie wstępne grupowanie po rozmiarze z weryfikacją sumą skrótu SHA-256.
- **Dzielenie i łączenie dużych plików (Split & Join):**
  - Dzielenie na części o zadanym rozmiarze (predefiniowane: dyskietka 1.44MB, Zip 100MB, CD 650MB/700MB, DVD 4.7GB lub własny rozmiar).
  - Generowanie pliku sum kontrolnych `.crc` (standard sum kontrolnych CRC32).
  - Składanie plików z automatyczną weryfikacją integralności CRC32.
- **Kodowanie i dekodowanie:**
  - **UUE** (Unix-to-Unix Encode / UUDecode),
  - **XXE** (XXEncode / XXDecode odporne na bramki pocztowe i systemy EBCDIC),
  - **MIME Base64** z zawijaniem do 76 znaków.
- **Synchronizacja katalogów:**
  - Porównywanie dwóch drzew katalogowych i klasyfikacja stanów (`IDENTICAL`, `LEFT_NEWER`, `RIGHT_NEWER`, `LEFT_ONLY`, `RIGHT_ONLY`, `SIZE_DIFF`).
  - Elastyczne akcje synchronizacji (kopiowanie lewo->prawo, prawo->lewo, usuwanie osieroconych plików).
- **Zaawansowana wyszukiwarka (`Ctrl+F`):**
  - Wyszukiwanie po wzorcu nazwy (wildcards/glob),
  - Filtrowanie po dacie modyfikacji i rozmiarze,
  - Przeszukiwanie pełnotekstowe wewnątrz plików tekstowych.

### 🔌 Transmisja szeregowa (Serial Port)
- Wbudowana implementacja protokołów transferu plików przez port szeregowy:
  - **XMODEM** (pakiety 128B, tryby sumy kontrolnej i CRC-16),
  - **YMODEM** (pakiety 1KB STX, przesyłanie nagłówka metadanych pliku).

### 👁️ Wbudowane narzędzia tekstowe
- **Podgląd plików (Lister / `F3`):** Szybki podgląd plików tekstowych i binarnych z przewijaniem i numeracją wierszy.
- **Edytor plików (Editor / `F4`):** Wbudowany edytor tekstu z nawigacją kursorem, edycją treści i zapisem pod `Ctrl+S`.

---

## ⌨️ Skróty klawiszowe

| Klawisz | Akcja |
|---|---|
| `F1` | Pomoc i wykaz skrótów klawiszowych |
| `F2` | Przełączanie podziału paneli (poziomy / pionowy) |
| `F3` | Podgląd pliku (Lister) |
| `F4` | Edycja pliku (Editor) |
| `F5` | Kopiowanie pliku lub zaznaczonych elementów |
| `F6` | Zmiana nazwy / przenoszenie elementów |
| `F7` | Tworzenie nowego katalogu (Mkdir) |
| `F8` / `Delete` | Usuwanie pliku lub zaznaczonych elementów |
| `F9` | Górne menu programu |
| `F10` | Wyjście z programu |
| `Tab` | Przełączenie aktywnego panelu (lewy ↔ prawy) |
| `Spacja` / `Insert` | Zaznaczenie / odznaczenie elementu pod kursorem |
| `+` | Zaznacz pliki według wzorca (np. `*.go`) |
| `-` | Odznacz pliki według wzorca |
| `*` | Odwróć zaznaczenie w aktywnym panelu |
| `Ctrl+A` | Zaznacz wszystkie elementy |
| `Ctrl+F` | Wyszukiwarka plików |
| `Ctrl+R` | Odświeżenie zawartości obu paneli |
| `Ctrl+U` | Zmiana orientacji paneli (horyzontalny / wertykalny) |
| `Enter` | Wejście do katalogu lub archiwum (ZIP / TAR / TGZ / BZ2) / uruchomienie |
| `Strzałki G/D` | Poruszanie kursorem po liście |
| `PageUp / PageDn` | Szybkie przewijanie strony listy plików |
| `Home / End` | Skok na początek / koniec listy |

---

## 📁 Struktura projektu

```text
myc/
├── cmd/
│   └── myc/                   # Punkt wejścia aplikacji (main.go)
├── internal/
│   ├── operations/            # Logika operacji na plikach
│   │   ├── compare.go         # Porównywanie zawartości (diff)
│   │   ├── duplicates.go      # Wykrywanie duplikatów (SHA-256)
│   │   ├── encode.go          # Kodery UUE, XXE, MIME Base64
│   │   ├── fileops.go         # Kopiowanie, przenoszenie, usuwanie
│   │   ├── rename.go          # Masowa zmiana nazw (Multi-Rename)
│   │   ├── search.go          # Zaawansowana wyszukiwarka plików
│   │   ├── serial.go          # Protokoły transmisji XMODEM i YMODEM
│   │   ├── splitjoin.go       # Dzielenie i łączenie plików + CRC32
│   │   └── sync.go            # Synchronizacja katalogów
│   ├── ui/                    # Warstwa interfejsu TUI (tcell/v2)
│   │   ├── app.go             # Główna pętla aplikacji i obsługa zdarzeń
│   │   ├── commandbar.go      # Wiersz poleceń
│   │   ├── dialogs.go         # Okna dialogowe (Input, Confirm)
│   │   ├── editor.go          # Wbudowany edytor tekstu (F4)
│   │   ├── help.go            # Okno pomocy (F1)
│   │   ├── keybar.go          # Dolny pasek klawiszy funkcyjnych F1-F10
│   │   ├── lister.go          # Wbudowana przeglądarka plików (F3)
│   │   ├── panel.go           # Komponent panelu listy plików
│   │   ├── render_helpers.go  # Narzędzia pomocnicze rysowania ramek
│   │   └── theme.go           # Palety kolorów (Classic Blue, Midnight)
│   ├── version/               # Informacje o wersji programu
│   └── vfs/                   # Warstwa wirtualnego systemu plików
│       ├── local.go           # VFS dla lokalnego systemu plików
│       ├── tar.go             # VFS dla archiwów TAR, TAR.GZ, TAR.BZ2
│       ├── vfs.go             # Interfejs VFS i model wpisu FileEntry
│       └── zip.go             # VFS dla archiwów ZIP
├── go.mod
├── go.sum
├── LICENSE
└── README.md
```

---

## 🔨 Budowanie i instalacja

Do zbudowania projektu wymagany jest kompilator **Go 1.24+**.

### Budowanie natywne (Linux / macOS)
```bash
# Sklonuj repozytorium
git clone https://github.com/pawel-laba/myc.git
cd myc

# Uruchomienie testów jednostkowych
go test -v ./...

# Kompilacja pliku wykonywalnego
go build -o myc ./cmd/myc

# Uruchomienie w trybie okienkowym Desktop
./myc --gui

# Uruchomienie w trybie konsolowym Terminal (TUI)
./myc --cli
```

### Kompilacja wieloplatformowa (Cross-compilation)
Projekt nie posiada zewnętrznych zależności CGo, co pozwala na bezproblemową kompilację krzyżową na dowolne architektury:

```bash
# Linux 64-bit (x86_64)
GOOS=linux GOARCH=amd64 go build -o myc-linux-amd64 ./cmd/myc

# Linux 32-bit (x86)
GOOS=linux GOARCH=386 go build -o myc-linux-386 ./cmd/myc

# Windows 64-bit
GOOS=windows GOARCH=amd64 go build -o myc-windows-amd64.exe ./cmd/myc

# Windows 32-bit
GOOS=windows GOARCH=386 go build -o myc-windows-386.exe ./cmd/myc
```

---

## 🧪 Testy

Wszystkie moduły logiczne posiadają dedykowane testy jednostkowe:
```bash
go test -v ./...
```

---

## 📄 Licencja

Projekt dystrybuowany na warunkach licencji **MIT**. Szczegóły w pliku [LICENSE](LICENSE).
