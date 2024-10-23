package main

import "time"

type templater struct {
	journalEntries []journal
}

type journal struct {
	Date time.Time
	URL  string
}

func newTemplater() (*templater, error) {
	je, err := loadJournal()
	if err != nil {
		return nil, err
	}

	return &templater{journalEntries: je}, nil
}

func loadJournal() ([]journal, error) {
	return nil, nil
}
