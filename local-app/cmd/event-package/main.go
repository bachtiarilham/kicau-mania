// event-package generates explicitly marked demonstration packages for offline tests.
// Production bundles should be exported by the cloud event preparation workflow.
package main

import (
	"crypto/rand"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/crypto/bcrypt"

	"kicau.local/edge/internal/core/arena/model"
	"kicau.local/edge/internal/core/identifier"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
func run() error {
	demo := flag.Bool("demo", false, "required: generate synthetic participants, not a real registration")
	out := flag.String("out", "./data/demo-package", "private output directory")
	arena := flag.String("arena", "arena01", "arena identity")
	engine := flag.String("engine", "engine01", "engine identity")
	id := flag.String("event", "demo01", "event identity")
	judges := flag.Int("judges", 2, "judges per round")
	rounds := flag.Int("rounds", 1, "number of prepared rounds")
	criteria := flag.String("criteria", "irama,volume,gaya", "comma separated criterion IDs")
	flag.Parse()
	if !*demo {
		return fmt.Errorf("use -demo for synthetic test data; real events must use a prepared cloud bundle")
	}
	if !identifier.Valid(*id) || !identifier.Valid(*arena) || !identifier.Valid(*engine) || *judges < 1 || *judges > 256 || *rounds < 1 || *rounds > 256 {
		return fmt.Errorf("invalid identities or counts")
	}
	b := model.Bundle{ID: *id, Name: "DEMO — bukan event resmi", Version: 1, ArenaID: *arena, EngineID: *engine}
	seen := map[string]bool{}
	for _, id := range strings.Split(*criteria, ",") {
		id = strings.TrimSpace(id)
		if !identifier.Valid(id) || seen[id] {
			return fmt.Errorf("invalid criterion")
		}
		seen[id] = true
		b.Criteria = append(b.Criteria, model.Criterion{ID: id, Name: id})
	}
	credentials := [][]string{{"event_id", "username", "password"}}
	for i := 1; i <= *judges; i++ {
		var random [16]byte
		if _, err := rand.Read(random[:]); err != nil {
			return err
		}
		password := hex.EncodeToString(random[:])
		hash, err := bcrypt.GenerateFromPassword([]byte(password), 12)
		if err != nil {
			return err
		}
		id := fmt.Sprintf("juri%02d", i)
		b.Judges = append(b.Judges, model.Judge{ID: id, Name: fmt.Sprintf("Juri %d", i), Username: id, PasswordHash: string(hash)})
		credentials = append(credentials, []string{b.ID, id, password})
	}
	for i := 1; i <= *rounds; i++ {
		r := model.Round{ID: fmt.Sprintf("ronde%02d", i), Name: fmt.Sprintf("Ronde %d", i)}
		for j := 1; j <= 24; j++ {
			r.Participants = append(r.Participants, model.Participant{ID: fmt.Sprintf("peserta-%02d-%02d", i, j), Name: fmt.Sprintf("Peserta demo %d", j), Lapak: j, ChannelID: fmt.Sprintf("lapak%02d", j)})
		}
		b.Rounds = append(b.Rounds, r)
	}
	if err := os.MkdirAll(*out, 0700); err != nil {
		return err
	}
	bundlePath, credentialPath := filepath.Join(*out, "event.json"), filepath.Join(*out, "judge-credentials.csv")
	for _, p := range []string{bundlePath, credentialPath} {
		if _, err := os.Stat(p); !os.IsNotExist(err) {
			return fmt.Errorf("output exists or cannot be checked; choose a new directory")
		}
	}
	data, err := json.MarshalIndent(b, "", "  ")
	if err != nil {
		return err
	}
	f, err := os.OpenFile(bundlePath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	if _, err = f.Write(data); err != nil {
		f.Close()
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	f, err = os.OpenFile(credentialPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	w := csv.NewWriter(f)
	w.WriteAll(credentials)
	writeErr := w.Error()
	closeErr := f.Close()
	if writeErr != nil {
		return writeErr
	}
	if closeErr != nil {
		return closeErr
	}
	fmt.Println("Demo package and private judge credentials created. Passwords are not printed.")
	return nil
}
