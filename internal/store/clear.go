package store

// ... existing code ...

// ClearMinionState deletes all notification records for a specific minion.
func (s *Store) ClearMinionState(minionName string) (int64, error) {
	res, err := s.db.Exec("DELETE FROM sent_notifications WHERE minion_name = ?", minionName)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// ClearAllState deletes all notification records for all minions.
func (s *Store) ClearAllState() (int64, error) {
	res, err := s.db.Exec("DELETE FROM sent_notifications")
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}