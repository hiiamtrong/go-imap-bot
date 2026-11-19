-- Keep the mail body so a transaction can be traced back to the mail it came from
ALTER TABLE mails ADD COLUMN body TEXT DEFAULT '';
