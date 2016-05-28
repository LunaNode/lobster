package lobster

// database objects

type SSHKey struct {
	ID     int
	UserID int
	Name   string
	Key    string
}

func keyListHelper(rows Rows) []*SSHKey {
	defer rows.Close()
	keys := make([]*SSHKey, 0)
	for rows.Next() {
		key := SSHKey{}
		rows.Scan(&key.ID, &key.UserID, &key.Name, &key.Key)
		keys = append(keys, &key)
	}
	return keys
}

const SSHKEY_QUERY = "SELECT id, user_id, name, val FROM sshkeys"

func keyListAll() []*SSHKey {
	return keyListHelper(
		db.Query(
			SSHKEY_QUERY + " ORDER BY user_id, name",
		),
	)
}

func keyList(userID int) []*SSHKey {
	return keyListHelper(
		db.Query(
			SSHKEY_QUERY+" WHERE user_id = ? ORDER BY name",
			userID,
		),
	)
}

func keyGet(userID int, id int) *SSHKey {
	keys := keyListHelper(
		db.Query(
			SSHKEY_QUERY+" WHERE id = ? AND user_id = ?",
			id, userID,
		),
	)
	if len(keys) == 1 {
		return keys[0]
	} else {
		return nil
	}
}

func keyAdd(userID int, name string, key string) (int, error) {
	if name == "" {
		return 0, L.Error("name_empty")
	} else if key == "" {
		return 0, L.Error("key_empty")
	}

	result := db.Exec(
		"INSERT INTO sshkeys (user_id, name, val) VALUES (?, ?, ?)",
		userID, name, key,
	)
	return result.LastInsertId(), nil
}

func keyRemove(userID int, id int) error {
	db.Exec("DELETE FROM sshkeys WHERE user_id = ? AND id = ?", userID, id)
	return nil
}
