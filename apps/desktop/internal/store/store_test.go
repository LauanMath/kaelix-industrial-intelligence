package store

import (
	"context"
	"path/filepath"
	"testing"
)

func TestProfileSurvivesReopen(t *testing.T) {
	path := filepath.Join(t.TempDir(), "kaelix.db")
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	p, err := s.SaveProfile(ctx, Profile{Name: " Ana ", Email: "ana@example.com", Company: "Kaelix", JobTitle: "Operadora"})
	if err != nil {
		t.Fatal(err)
	}
	if p.Name != "Ana" || p.UpdatedAt == "" {
		t.Fatalf("unexpected profile: %+v", p)
	}
	s.Close()
	s, err = Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	loaded, err := s.Profile(ctx)
	if err != nil || loaded != p {
		t.Fatalf("reopen: %+v, %v", loaded, err)
	}
	for _, invalid := range []Profile{{Name: ""}, {Name: "Ana", Email: "bad"}, {Name: "Ana", Email: "Ana <ana@example.com>"}} {
		if _, err = s.SaveProfile(ctx, invalid); err == nil {
			t.Fatal("accepted invalid profile")
		}
	}
	loaded, _ = s.Profile(ctx)
	if loaded != p {
		t.Fatal("invalid update changed stored profile")
	}
}

func TestAvatarSurvivesProfileUpdate(t *testing.T) {
	path := filepath.Join(t.TempDir(), "profile.db")
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if _, err = s.SaveAvatar(ctx, "data:image/png;base64,test"); err != nil {
		t.Fatal(err)
	}
	if _, err = s.SaveProfile(ctx, Profile{Name: "Ana"}); err != nil {
		t.Fatal(err)
	}
	s.Close()
	s, err = Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	p, err := s.Profile(ctx)
	if err != nil || p.AvatarData != "data:image/png;base64,test" {
		t.Fatalf("photo lost: %+v %v", p, err)
	}
	if _, err = s.SaveAvatar(ctx, ""); err != nil {
		t.Fatal(err)
	}
	p, _ = s.Profile(ctx)
	if p.AvatarData != "" {
		t.Fatal("photo was not removed")
	}
}
