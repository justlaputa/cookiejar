// extend http.cookiejar to use different backend service as
// persistent storage

package cookiejar

import (
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"time"
)

// SetCookies implements cookiejar.SetCookies, delegate to http.cookiejar.SetCookies
//at the same time, write cookie to firestore
func (j *PersistentJar) SetCookies(u *url.URL, cookies []*http.Cookie) {
	log.Printf("SetCookies: set for url: %s, cookies: %d", u.String(), len(cookies))

	j.jar.SetCookies(u, cookies)
	err := j.saveToStorage(u)
	if err != nil {
		log.Printf("failed to save cookie to firestore, ignore. %v", err)
	}
}

// Cookies implements cookiejar.Cookies, delegate to http.cookiejar
// if no value found, get it from firestore
func (j *PersistentJar) Cookies(u *url.URL) []*http.Cookie {
	cookies := j.jar.Cookies(u)

	log.Printf("Cookies: get cookies for url: %s, found %d in memory", u.String(), len(cookies))

	if cookies == nil || len(cookies) == 0 {
		log.Printf("not found in memory, try persistent storage")
		c, err := j.loadFromStorage(u)
		if err != nil {
			log.Printf("failed to load cookie from firestore, ignore. %v", err)
			cookies = []*http.Cookie{}
		} else {
			log.Printf("found %d from persistent storage", len(c))
			j.jar.setCookies(u, c, time.Now())
			cookies = c
		}
	}

	return cookies
}

func (j *PersistentJar) loadFromStorage(u *url.URL) ([]*http.Cookie, error) {
	key, err := getKey(u, j.psl)
	if err != nil {
		return nil, err
	}

	submap, err := j.backend.load(key)
	if err != nil {
		return nil, err
	}

	cookies := []*http.Cookie{}
	for _, e := range submap {
		cookies = append(cookies, &http.Cookie{Name: e.Name, Value: e.Value})
	}

	return cookies, nil
}

func (j *PersistentJar) saveToStorage(u *url.URL) error {
	key, err := getKey(u, j.psl)
	if err != nil {
		return err
	}

	return j.backend.save(key, j.jar.entries[key])
}

//
func (j *PersistentJar) LoadFromFile(filename string) error {
	if _, err := os.Stat(filepath.Dir(filename)); os.IsNotExist(err) {
		log.Printf("cookie file not exist, skip load it. %v", err)
		return nil
	}

	f, err := os.Open(filename)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	defer f.Close()

	decoder := json.NewDecoder(f)
	// Cope with old cookiejar format by just discarding
	// cookies, but still return an error if it's invalid JSON.
	var data json.RawMessage
	if err := decoder.Decode(&data); err != nil {
		if err == io.EOF {
			// Empty file.
			return nil
		}
		return err
	}
	var entries []entry
	if err := json.Unmarshal(data, &entries); err != nil {
		log.Printf("warning: discarding cookies in invalid format (error: %v)", err)
		return nil
	}

	for _, e := range entries {
		if e.Domain == "" {
			continue
		}
		host, err := canonicalHost(e.Domain)
		if err != nil {
			log.Printf("failed to get host from domain: %s, error: %v", e.Domain, err)
			continue
		}
		key := jarKey(host, j.psl)
		id := e.id()
		submap := j.jar.entries[key]
		if submap == nil {
			j.jar.entries[key] = map[string]entry{
				id: e,
			}
			continue
		}

		submap[id] = e
	}

	return nil
}

//
func (j *PersistentJar) Save() {
	for key, submap := range j.jar.entries {
		j.backend.save(key, submap)
	}
}

// PersistentJarOptions for initialize
type PersistentJarOptions struct {
	PublicSuffixList    PublicSuffixList
	GCPProjectID        string
	FireStoreCollection string
}

// PersistentJar cookiejar with firestore as storage
type PersistentJar struct {
	psl     PublicSuffixList
	jar     *Jar
	backend *storage
}

// NewPersistentJar create new PersistentJar
func NewPersistentJar(o *PersistentJarOptions) *PersistentJar {
	jar, err := New(&Options{PublicSuffixList: o.PublicSuffixList})

	if err != nil {
		log.Fatal(err)
	}

	s := newFirestore(o.GCPProjectID, o.FireStoreCollection)

	return &PersistentJar{
		o.PublicSuffixList, jar, s,
	}
}

func getKey(u *url.URL, psl PublicSuffixList) (string, error) {
	host, err := canonicalHost(u.Host)
	if err != nil {
		return "", err
	}

	key := jarKey(host, psl)
	if key == "" {
		return "", errors.New("failed to get key from url")
	}

	return key, nil
}
