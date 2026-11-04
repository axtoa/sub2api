package quality

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type InspectionQuestionTemplate struct {
	ID string `json:"id"`
	InspectionQuestion
}

type InspectionQuestionBank struct {
	Items   []InspectionQuestionTemplate `json:"items"`
	Version int                          `json:"version"`
}

func (b InspectionQuestionBank) validate() error {
	if b.Version < 1 || len(b.Items) > 100 {
		return invalid("题库最多保存 100 道题，请刷新后重试")
	}
	seen := map[uuid.UUID]bool{}
	for _, item := range b.Items {
		id, err := uuid.Parse(item.ID)
		if err != nil || id == uuid.Nil || seen[id] {
			return invalid("题目编号无效或重复")
		}
		seen[id] = true
		if err := validateInspectionQuestions([]InspectionQuestion{item.InspectionQuestion}); err != nil {
			return err
		}
	}
	raw, err := json.Marshal(b.Items)
	if err != nil {
		return err
	}
	if len(raw) > 256<<10 {
		return invalid("题库内容最多 256 KB，请缩短问题或参考答案")
	}
	return nil
}

func (s *Store) InspectionQuestions(ctx context.Context) (InspectionQuestionBank, error) {
	bank := InspectionQuestionBank{Items: []InspectionQuestionTemplate{}}
	var raw []byte
	err := s.DB.QueryRowContext(ctx, `SELECT items,version FROM quality_inspection_question_bank WHERE id=TRUE`).Scan(&raw, &bank.Version)
	if err == nil {
		err = json.Unmarshal(raw, &bank.Items)
	}
	return bank, err
}

func (s *Store) SaveInspectionQuestions(ctx context.Context, bank InspectionQuestionBank) (InspectionQuestionBank, error) {
	if err := bank.validate(); err != nil {
		return bank, err
	}
	if bank.Items == nil {
		bank.Items = []InspectionQuestionTemplate{}
	}
	raw, err := json.Marshal(bank.Items)
	if err != nil {
		return bank, err
	}
	err = s.DB.QueryRowContext(ctx, `UPDATE quality_inspection_question_bank SET items=$1,version=version+1,updated_at=NOW() WHERE id=TRUE AND version=$2 RETURNING version`, string(raw), bank.Version).Scan(&bank.Version)
	if errors.Is(err, sql.ErrNoRows) {
		err = conflict("题库已被其他窗口修改，请重新载入后再编辑")
	}
	return bank, err
}

func (h *Handler) InspectionQuestions(c *gin.Context) {
	if !authorized(c) {
		return
	}
	bank, err := h.Runner.Store.InspectionQuestions(c.Request.Context())
	if !respondError(c, err) {
		response.Success(c, bank)
	}
}

func (h *Handler) SaveInspectionQuestions(c *gin.Context) {
	if !authorized(c) {
		return
	}
	var bank InspectionQuestionBank
	if !bindLimit(c, &bank, 512<<10) {
		return
	}
	saved, err := h.Runner.Store.SaveInspectionQuestions(c.Request.Context(), bank)
	if !respondError(c, err) {
		response.Success(c, saved)
	}
}
