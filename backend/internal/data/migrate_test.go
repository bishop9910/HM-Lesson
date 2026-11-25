package data

import "testing"

func TestLoadMigrations(t *testing.T) {
	all, err := loadMigrations()
	if err != nil {
		t.Fatalf("loadMigrations: %v", err)
	}
	if len(all) != 10 {
		t.Fatalf("want 10 migrations, got %d", len(all))
	}
	if all[0].version != 1 || all[0].name != "init" {
		t.Fatalf("unexpected first migration: %+v", all[0])
	}
	if all[1].version != 2 || all[1].name != "teaching" {
		t.Fatalf("unexpected second migration: %+v", all[1])
	}
	if all[2].version != 3 || all[2].name != "import_batch" {
		t.Fatalf("unexpected third migration: %+v", all[2])
	}
	if all[5].version != 6 || all[5].name != "subscription_messages" {
		t.Fatalf("unexpected sixth migration: %+v", all[5])
	}
	if all[6].version != 7 || all[6].name != "booking_lesson" {
		t.Fatalf("unexpected seventh migration: %+v", all[6])
	}
	if all[7].version != 8 || all[7].name != "fix_booking_lesson_tables" {
		t.Fatalf("unexpected eighth migration: %+v", all[7])
	}
	if all[8].version != 9 || all[8].name != "guardian_passwords" {
		t.Fatalf("unexpected ninth migration: %+v", all[8])
	}
	if all[9].version != 10 || all[9].name != "system_account" {
		t.Fatalf("unexpected tenth migration: %+v", all[9])
	}
	for _, m := range all {
		if len(m.up) == 0 || len(m.down) == 0 {
			t.Fatalf("migration %d missing up/down", m.version)
		}
	}
}

func TestSplitSQL(t *testing.T) {
	in := "-- comment\nCREATE TABLE a (id int);\n\nCREATE TABLE b (\n  id int\n);\n"
	got := splitSQL(in)
	if len(got) != 2 {
		t.Fatalf("want 2 statements, got %d: %#v", len(got), got)
	}
	if got[0] != "CREATE TABLE a (id int);" {
		t.Fatalf("unexpected first statement: %q", got[0])
	}
}
