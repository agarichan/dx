package dburl

import "testing"

func TestParse_RoundTrip(t *testing.T) {
	raw := "postgresql+asyncpg://myapp:devpassword@localhost:5434/myapp"
	d, err := Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	if d.Scheme != "postgresql" || d.Driver != "asyncpg" {
		t.Fatalf("scheme/driver = %q/%q", d.Scheme, d.Driver)
	}
	if d.User != "myapp" || d.Password != "devpassword" {
		t.Fatalf("user/pass = %q/%q", d.User, d.Password)
	}
	if d.Host != "localhost" || d.Port != "5434" || d.Name != "myapp" {
		t.Fatalf("host/port/name = %q/%q/%q", d.Host, d.Port, d.Name)
	}
	if got := d.String(); got != raw {
		t.Fatalf("String() = %q, want %q", got, raw)
	}
}

func TestWithName(t *testing.T) {
	d, _ := Parse("postgresql+asyncpg://myapp:devpassword@localhost:5434/myapp")
	d2 := d.WithName("myapp_featx")
	if d2.Name != "myapp_featx" || d.Name != "myapp" {
		t.Fatalf("WithName mutated original or wrong: %q / %q", d2.Name, d.Name)
	}
	want := "postgresql+asyncpg://myapp:devpassword@localhost:5434/myapp_featx"
	if d2.String() != want {
		t.Fatalf("String() = %q, want %q", d2.String(), want)
	}
}

func TestParse_NoDriverNoQuery(t *testing.T) {
	d, err := Parse("postgresql://u:p@h:5432/db")
	if err != nil {
		t.Fatal(err)
	}
	if d.Driver != "" {
		t.Fatalf("driver = %q", d.Driver)
	}
	if d.String() != "postgresql://u:p@h:5432/db" {
		t.Fatalf("String() = %q", d.String())
	}
}
