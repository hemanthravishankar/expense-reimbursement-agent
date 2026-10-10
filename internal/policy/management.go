package policy

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

const (
	VersionStatusDraft     = "DRAFT"
	VersionStatusPublished = "PUBLISHED"
	VersionStatusRetired   = "RETIRED"
)

var (
	ErrPolicyNotFound      = errors.New("policy version not found")
	ErrNoActivePolicy      = errors.New("no active reimbursement policy is configured")
	ErrPolicyNotDraft      = errors.New("only draft policy versions can be modified or published")
	ErrPolicyNameRequired  = errors.New("policy name is required")
	ErrPolicyCurrency      = errors.New("policy currency is required")
	ErrPolicyCreator       = errors.New("policy created_by is required")
	ErrPolicyPublisher     = errors.New("policy published_by is required")
	ErrPolicyRulesRequired = errors.New("at least one policy rule is required")
	ErrInvalidPolicyRule   = errors.New("policy rule is invalid")
	ErrFuturePolicyPublish = errors.New("a future-dated policy cannot be published yet")
)

type Rule struct {
	EmployeeGrade string  `json:"employee_grade"`
	Category      string  `json:"category"`
	Allowed       bool    `json:"allowed"`
	MaxAmount     float64 `json:"max_amount"`
}

type Version struct {
	PolicyID      string     `json:"policy_id"`
	Name          string     `json:"name"`
	Version       int        `json:"version"`
	Currency      string     `json:"currency"`
	Status        string     `json:"status"`
	EffectiveFrom string     `json:"effective_from"`
	EffectiveTo   string     `json:"effective_to,omitempty"`
	CreatedBy     string     `json:"created_by"`
	CreatedAt     time.Time  `json:"created_at"`
	PublishedBy   string     `json:"published_by,omitempty"`
	PublishedAt   *time.Time `json:"published_at,omitempty"`
	Rules         []Rule     `json:"rules"`
}

type CreateVersionRequest struct {
	Name          string `json:"name"`
	Currency      string `json:"currency"`
	EffectiveFrom string `json:"effective_from"`
	CreatedBy     string `json:"created_by"`
	Rules         []Rule `json:"rules"`
}

type UpdateVersionRequest struct {
	Currency      string `json:"currency"`
	EffectiveFrom string `json:"effective_from"`
	Rules         []Rule `json:"rules"`
}

type PublishRequest struct {
	PublishedBy string `json:"published_by"`
}

type ManagementStore interface {
	ActivePolicy(
		ctx context.Context,
	) (Policy, error)

	ListPolicyVersions(
		ctx context.Context,
	) ([]Version, error)

	GetPolicyVersion(
		ctx context.Context,
		policyID string,
	) (*Version, error)

	CreatePolicyVersion(
		ctx context.Context,
		request CreateVersionRequest,
	) (*Version, error)

	UpdatePolicyVersion(
		ctx context.Context,
		policyID string,
		request UpdateVersionRequest,
	) (*Version, error)

	PublishPolicyVersion(
		ctx context.Context,
		policyID string,
		publishedBy string,
	) (*Version, error)
}

type Manager struct {
	store ManagementStore
}

func NewManager(
	store ManagementStore,
) *Manager {
	return &Manager{
		store: store,
	}
}

func (m *Manager) Active(
	ctx context.Context,
) (Policy, error) {
	if m == nil || m.store == nil {
		return Policy{}, fmt.Errorf("policy management store is not configured")
	}

	return m.store.ActivePolicy(ctx)
}

func (m *Manager) List(
	ctx context.Context,
) ([]Version, error) {
	if m == nil || m.store == nil {
		return nil, fmt.Errorf("policy management store is not configured")
	}

	return m.store.ListPolicyVersions(ctx)
}

func (m *Manager) Get(
	ctx context.Context,
	policyID string,
) (*Version, error) {
	if m == nil || m.store == nil {
		return nil, fmt.Errorf("policy management store is not configured")
	}

	policyID = strings.TrimSpace(policyID)
	if policyID == "" {
		return nil, ErrPolicyNotFound
	}

	return m.store.GetPolicyVersion(ctx, policyID)
}

func (m *Manager) Create(
	ctx context.Context,
	request CreateVersionRequest,
) (*Version, error) {
	if m == nil || m.store == nil {
		return nil, fmt.Errorf("policy management store is not configured")
	}

	normalizeCreateRequest(&request)

	if err := validateCreateRequest(request); err != nil {
		return nil, err
	}

	return m.store.CreatePolicyVersion(ctx, request)
}

func (m *Manager) Update(
	ctx context.Context,
	policyID string,
	request UpdateVersionRequest,
) (*Version, error) {
	if m == nil || m.store == nil {
		return nil, fmt.Errorf("policy management store is not configured")
	}

	policyID = strings.TrimSpace(policyID)
	if policyID == "" {
		return nil, ErrPolicyNotFound
	}

	normalizeUpdateRequest(&request)

	if err := validateUpdateRequest(request); err != nil {
		return nil, err
	}

	return m.store.UpdatePolicyVersion(
		ctx,
		policyID,
		request,
	)
}

func (m *Manager) Publish(
	ctx context.Context,
	policyID string,
	request PublishRequest,
) (*Version, error) {
	if m == nil || m.store == nil {
		return nil, fmt.Errorf("policy management store is not configured")
	}

	policyID = strings.TrimSpace(policyID)
	if policyID == "" {
		return nil, ErrPolicyNotFound
	}

	request.PublishedBy =
		strings.TrimSpace(request.PublishedBy)

	if request.PublishedBy == "" {
		return nil, ErrPolicyPublisher
	}

	return m.store.PublishPolicyVersion(
		ctx,
		policyID,
		request.PublishedBy,
	)
}

func normalizeCreateRequest(
	request *CreateVersionRequest,
) {
	request.Name = strings.TrimSpace(request.Name)
	request.Currency = strings.ToUpper(
		strings.TrimSpace(request.Currency),
	)
	request.EffectiveFrom = strings.TrimSpace(request.EffectiveFrom)
	request.CreatedBy = strings.TrimSpace(request.CreatedBy)
	request.Rules = normalizeRules(request.Rules)
}

func normalizeUpdateRequest(
	request *UpdateVersionRequest,
) {
	request.Currency = strings.ToUpper(
		strings.TrimSpace(request.Currency),
	)
	request.EffectiveFrom = strings.TrimSpace(request.EffectiveFrom)
	request.Rules = normalizeRules(request.Rules)
}

func normalizeRules(
	rules []Rule,
) []Rule {
	result := make([]Rule, 0, len(rules))

	for _, rule := range rules {
		rule.EmployeeGrade = strings.TrimSpace(rule.EmployeeGrade)
		rule.Category = strings.TrimSpace(rule.Category)
		result = append(result, rule)
	}

	return result
}

func validateCreateRequest(
	request CreateVersionRequest,
) error {
	if request.Name == "" {
		return ErrPolicyNameRequired
	}

	if request.Currency == "" {
		return ErrPolicyCurrency
	}

	if request.CreatedBy == "" {
		return ErrPolicyCreator
	}

	if err := validateDate(request.EffectiveFrom); err != nil {
		return err
	}

	return validateRules(request.Rules)
}

func validateUpdateRequest(
	request UpdateVersionRequest,
) error {
	if request.Currency == "" {
		return ErrPolicyCurrency
	}

	if err := validateDate(request.EffectiveFrom); err != nil {
		return err
	}

	return validateRules(request.Rules)
}

func validateDate(value string) error {
	if value == "" {
		return fmt.Errorf("effective_from is required")
	}

	if _, err := time.Parse("2006-01-02", value); err != nil {
		return fmt.Errorf("effective_from must use YYYY-MM-DD format")
	}

	return nil
}

func validateRules(
	rules []Rule,
) error {
	if len(rules) == 0 {
		return ErrPolicyRulesRequired
	}

	seen := map[string]struct{}{}

	for _, rule := range rules {
		if rule.EmployeeGrade == "" || rule.Category == "" {
			return fmt.Errorf(
				"%w: employee_grade and category are required",
				ErrInvalidPolicyRule,
			)
		}

		if rule.Allowed && rule.MaxAmount <= 0 {
			return fmt.Errorf(
				"%w: allowed rule %s/%s must have max_amount greater than zero",
				ErrInvalidPolicyRule,
				rule.EmployeeGrade,
				rule.Category,
			)
		}

		if rule.MaxAmount < 0 {
			return fmt.Errorf(
				"%w: max_amount cannot be negative",
				ErrInvalidPolicyRule,
			)
		}

		key := strings.ToUpper(rule.EmployeeGrade) + "\x00" + strings.ToLower(rule.Category)
		if _, exists := seen[key]; exists {
			return fmt.Errorf(
				"%w: duplicate rule for %s/%s",
				ErrInvalidPolicyRule,
				rule.EmployeeGrade,
				rule.Category,
			)
		}

		seen[key] = struct{}{}
	}

	return nil
}
