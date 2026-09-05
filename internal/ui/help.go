package ui

import (
	"strings"
)

// ShowHelpDialog creates a comprehensive help message dialog.
func ShowHelpDialog(onDismiss func()) *Dialog {
	helpText := strings.Join([]string{
		"MYC - Menadżer Plików (Norton/Total Commander dla Win/Linux)",
		"-------------------------------------------------------------",
		"Nawigacja:",
		"  Strzałki Góra/Dół   - Przewijanie listy plików",
		"  Tab                 - Przełączanie między lewym i prawym panelem",
		"  Enter               - Wejście do katalogu / otwarcie archiwum .ZIP",
		"  Spacja / Insert     - Zaznaczenie / odznaczenie pliku",
		"  Ctrl+A              - Zaznacz wszystkie pliki",
		"  Ctrl+U              - Przełącz układ (pionowy / poziomy)",
		"  Home / End          - Skok na początek / koniec listy",
		"  PgUp / PgDn         - Przewijanie stronami",
		"",
		"Klawisze funkcyjne:",
		"  F1 Pomoc      - Wyświetlenie tego okna pomocy",
		"  F2 Układ      - Przełączanie podziału (pionowo / poziomo)",
		"  F3 Podgląd    - Szybki podgląd zawartości (tekst / HEX)",
		"  F4 Edycja     - Wbudowany edytor tekstu",
		"  F5 Kopiuj     - Kopiowanie zaznaczonych plików do drugiego panelu",
		"  F6 Zmień/Przen- Zmiana nazwy lub przeniesienie pliku",
		"  F7 NowyKat    - Utworzenie nowego katalogu",
		"  F8 Usuń       - Usunięcie zaznaczonych plików / katalogów",
		"  F9 Narzędzia  - Narzędzie masowej zmiany nazw, duplikaty, diff",
		"  F10 Wyjście   - Zakończenie pracy programu",
		"",
		"Wiersz poleceń na dole:",
		"  Wpisz dowolną komendę systemową i naciśnij Enter.",
	}, "\n")

	return NewMessageDialog("POMOC - SKRÓTY KLAWISZOWE", helpText, onDismiss)
}
