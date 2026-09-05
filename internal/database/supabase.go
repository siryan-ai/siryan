package database

import (
	"github.com/supabase-community/supabase-go"
)

func NewSupabaseClient(url, serviceKey string) (*supabase.Client, error) {
	client, err := supabase.NewClient(url, serviceKey, nil)
	if err != nil {
		return nil, err
	}
	return client, nil
}
