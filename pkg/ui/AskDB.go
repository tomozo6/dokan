package ui

func AskDB(DBNames []string) (*string, error) {
	return askSelection("Choose Target DB:", DBNames)
}
