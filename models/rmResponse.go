package models

type RmResponse struct {
	CaseID          string `json:"caseId,omitempty"`
	QuestionnaireID string `json:"questionnaireId"`
	Unreceipt       bool   `json:"unreceipt"`
}
