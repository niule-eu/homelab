package effects

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFileWrite(t *testing.T) {
	t.Run("writes content to file", func(t *testing.T) {
		tmp := t.TempDir()
		path := filepath.Join(tmp, "test.txt")
		content := []byte("hello world")

		err := FileWrite{Path: path, Content: content, Permissions: 0644}.Apply()
		if err != nil {
			t.Fatalf("Apply() failed: %v", err)
		}

		got, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if string(got) != string(content) {
			t.Fatalf("expected %q, got %q", content, got)
		}
	})

	t.Run("respects permissions", func(t *testing.T) {
		tmp := t.TempDir()
		path := filepath.Join(tmp, "restricted.txt")

		err := FileWrite{Path: path, Content: []byte("secret"), Permissions: 0600}.Apply()
		if err != nil {
			t.Fatal(err)
		}

		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode().Perm() != 0600 {
			t.Fatalf("expected permissions 0600, got %v", info.Mode().Perm())
		}
	})
}

func TestFileDelete(t *testing.T) {
	t.Run("deletes existing file", func(t *testing.T) {
		tmp := t.TempDir()
		path := filepath.Join(tmp, "delete.txt")
		if err := os.WriteFile(path, []byte("x"), 0644); err != nil {
			t.Fatal(err)
		}

		err := FileDelete{Path: path}.Apply()
		if err != nil {
			t.Fatalf("Apply() failed: %v", err)
		}

		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatal("expected file to be deleted")
		}
	})

	t.Run("returns error for missing file", func(t *testing.T) {
		err := FileDelete{Path: "/nonexistent/file.txt"}.Apply()
		if err == nil {
			t.Fatal("expected error for missing file")
		}
	})
}

func TestStdout(t *testing.T) {
	t.Run("prints message without error", func(t *testing.T) {
		err := Stdout{Message: "test"}.Apply()
		if err != nil {
			t.Fatalf("Apply() returned error: %v", err)
		}
	})
}

func TestNoOp(t *testing.T) {
	t.Run("does nothing", func(t *testing.T) {
		err := NoOp{}.Apply()
		if err != nil {
			t.Fatalf("Apply() returned error: %v", err)
		}
	})
}

func TestCompound(t *testing.T) {
	t.Run("executes all effects", func(t *testing.T) {
		tmp := t.TempDir()
		path1 := filepath.Join(tmp, "a.txt")
		path2 := filepath.Join(tmp, "b.txt")

		compound := Compound{
			Effects: []Effect{
				FileWrite{Path: path1, Content: []byte("a"), Permissions: 0644},
				FileWrite{Path: path2, Content: []byte("b"), Permissions: 0644},
			},
		}

		if err := compound.Apply(); err != nil {
			t.Fatalf("Apply() failed: %v", err)
		}

		if _, err := os.Stat(path1); err != nil {
			t.Error("file a.txt not created")
		}
		if _, err := os.Stat(path2); err != nil {
			t.Error("file b.txt not created")
		}
	})

	t.Run("collects all errors by default", func(t *testing.T) {
		compound := Compound{
			Effects: []Effect{
				FileDelete{Path: "/no/such/file/1"},
				FileDelete{Path: "/no/such/file/2"},
			},
		}

		err := compound.Apply()
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("fails fast when configured", func(t *testing.T) {
		tmp := t.TempDir()
		path := filepath.Join(tmp, "ok.txt")

		compound := Compound{
			Effects: []Effect{
				FileWrite{Path: path, Content: []byte("ok"), Permissions: 0644},
				FileDelete{Path: "/no/such/file"},
				FileWrite{Path: path, Content: []byte("never"), Permissions: 0644},
			},
			FailFast: true,
		}

		_ = compound.Apply()

		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if string(content) == "never" {
			t.Fatal("third effect should not have executed")
		}
	})
}

func TestInvoke(t *testing.T) {
	t.Run("executes variadic effects", func(t *testing.T) {
		tmp := t.TempDir()
		path := filepath.Join(tmp, "invoke.txt")

		err := Invoke(
			false,
			FileWrite{Path: path, Content: []byte("invoked"), Permissions: 0644},
		)
		if err != nil {
			t.Fatalf("Invoke() failed: %v", err)
		}

		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if string(content) != "invoked" {
			t.Fatalf("expected 'invoked', got %q", content)
		}
	})
}
