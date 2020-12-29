// storage implements backend persistent storage, now use firestore

package cookiejar

import (
	"context"
	"log"

	"cloud.google.com/go/firestore"
)

// type fscookie struct {
// 	Name       string    `firestore:"name"`
// 	Value      string    `firestore:"value"`
// 	Domain     string    `firestore:"domain"`
// 	Path       string    `firestore:"path"`
// 	Expires    time.Time `firestore:"expires"`
// 	HttpOnly   bool      `firestore:"httponly"`
// 	Secure     bool      `firestore:"secure"`
// 	SameSite   string    `firestore:"samesite"`
// 	Persistent bool      `firestore:"persistent"`
// }

// type fsCookie entry

// type fsDoc []fsCookie

type storage struct {
	fsContext  context.Context
	fsClient   *firestore.Client
	collection string
}

func (s *storage) save(key string, data map[string]entry) error {
	log.Printf("save entries to storage, doc name: %s, entries: %d", key, len(data))

	docName := key

	doc := s.fsClient.Collection(s.collection).Doc(docName)

	_, err := doc.Set(s.fsContext, data)
	if err != nil {
		log.Printf("failed to set doc, %v", err)
	}

	return nil
}

func (s *storage) load(key string) (map[string]entry, error) {
	docName := key

	data := make(map[string]entry)

	docsnap, err := s.fsClient.Collection(s.collection).Doc(docName).Get(s.fsContext)
	if err != nil {
		return nil, err
	}

	if err = docsnap.DataTo(&data); err != nil {
		log.Printf("failed to read doc to variable, %v", err)
		return nil, err
	}

	return data, nil
}

func newFirestore(projectID string, collectionName string) *storage {
	ctx := context.Background()
	client, err := firestore.NewClient(ctx, projectID)
	if err != nil {
		log.Fatal(err)
	}

	return &storage{
		ctx, client, collectionName,
	}
}
