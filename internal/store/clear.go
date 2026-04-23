package store

// ClearMinionState deletes all notification and dropped URL records for a specific minion.
func (s *Store) ClearMinionState(minionName string) (int64, error) {
	res1, err := s.db.Exec("DELETE FROM sent_notifications WHERE minion_name = ?", minionName)
	if err != nil {
		return 0, err
	}
	res2, err := s.db.Exec("DELETE FROM dropped_urls WHERE minion_name = ?", minionName)
	if err != nil {
		return 0, err
	}

	count1, _ := res1.RowsAffected()
	count2, _ := res2.RowsAffected()
	return count1 + count2, nil
}

// ClearAllState deletes all notification and dropped URL records for all minions.
func (s *Store) ClearAllState() (int64, error) {
	res1, err := s.db.Exec("DELETE FROM sent_notifications")
	if err != nil {
		return 0, err
	}
	res2, err := s.db.Exec("DELETE FROM dropped_urls")
	if err != nil {
		return 0, err
	}

	count1, _ := res1.RowsAffected()
	count2, _ := res2.RowsAffected()
	return count1 + count2, nil
}
