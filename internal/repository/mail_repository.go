package repository

import (
	"database/sql"
	"fmt"
	"strings"

	"github.com/hiiamtrong/imap-bot-go/internal/database"
	"github.com/hiiamtrong/imap-bot-go/internal/models"
)

type MailRepository struct {
	db *database.Database
}

func NewMailRepository(db *database.Database) *MailRepository {
	return &MailRepository{db: db}
}

func (r *MailRepository) Create(mail *models.Mail) error {
	query := `
		INSERT INTO mails (uid, subject, from_account, to_account, timestamp)
		VALUES (?, ?, ?, ?, ?)
	`
	result, err := r.db.Conn.Exec(query, mail.UID, mail.Subject, mail.From, mail.To, mail.Date.Unix())
	if err != nil {
		return fmt.Errorf("failed to insert mail: %v", err)
	}
	fmt.Printf("Mail inserted successfully: %v\n, err: %v\n", mail, err)

	id, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("failed to get last insert id: %v", err)
	}

	mail.ID = id
	return nil
}

func (r *MailRepository) GetByUID(uid int64) (*models.Mail, error) {
	query := `
		SELECT id, uid, subject, from_account, to_account, timestamp
		FROM mails
		WHERE uid = ?
	`
	mail := &models.Mail{}
	var timestamp int64
	err := r.db.Conn.QueryRow(query, uid).Scan(
		&mail.ID,
		&mail.UID,
		&mail.Subject,
		&mail.From,
		&mail.To,
		&timestamp,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get mail: %v", err)
	}

	mail.Date = models.UnixToTime(timestamp)
	return mail, nil
}

func (r *MailRepository) GetNonProcessedMails(uids []int64) ([]int64, error) {
	// Remove the uids that are already processed

	mapUids := make(map[int64]bool)
	for _, uid := range uids {
		mapUids[uid] = true
	}

	placeholders := make([]string, len(uids))
	args := make([]interface{}, len(uids))
	for i := range uids {
		placeholders[i] = "?"
		args[i] = uids[i]
	}

	query := fmt.Sprintf(`
		SELECT uid
		FROM mails
		WHERE uid IN (%s)
	`, strings.Join(placeholders, ","))

	rows, err := r.db.Conn.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to get non processed mails: %v", err)
	}
	defer rows.Close()

	nonProcessedMails := make([]int64, 0)
	for rows.Next() {
		var uid int64
		err = rows.Scan(&uid)
		if err != nil {
			return nil, fmt.Errorf("failed to scan mail: %v", err)
		}
		mapUids[uid] = false
	}

	for uid, processed := range mapUids {
		if processed {
			nonProcessedMails = append(nonProcessedMails, uid)
		}
	}

	return nonProcessedMails, nil
}
