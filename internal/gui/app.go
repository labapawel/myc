package gui

import (
	"fmt"
	"os"
	"time"
)

// RunDesktop starts the myc GUI server and opens the Total Commander desktop window.
func RunDesktop(leftPath, rightPath string) error {
	srv, err := NewServer(leftPath, rightPath)
	if err != nil {
		return fmt.Errorf("błąd inicjalizacji serwera GUI: %w", err)
	}

	go func() {
		if err := srv.Start(); err != nil && err.Error() != "http: Server closed" {
			fmt.Fprintf(os.Stderr, "Błąd serwera GUI: %v\n", err)
		}
	}()
	defer srv.Close()

	url := srv.URL()
	fmt.Printf("🚀 Uruchamianie myc w trybie Desktop (Total Commander)...\n")
	fmt.Printf("   Adres URL: %s\n", url)

	browserCmd, err := LaunchAppWindow(url)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Ostrzeżenie: %v\nMożesz otworzyć interfejs ręcznie pod adresem: %s\n", err, url)
	}

	// Wait for exit signal or browser process exit
	if browserCmd != nil && browserCmd.Process != nil {
		done := make(chan error, 1)
		go func() {
			done <- browserCmd.Wait()
		}()

		select {
		case <-srv.StopChan():
			// Clean exit from Web UI
			if browserCmd.Process != nil {
				_ = browserCmd.Process.Kill()
			}
		case <-done:
			// Browser window was closed by user
		}
	} else {
		// Wait on stop channel
		<-srv.StopChan()
	}

	time.Sleep(100 * time.Millisecond)
	fmt.Println("Zamknięto myc Desktop.")
	return nil
}
