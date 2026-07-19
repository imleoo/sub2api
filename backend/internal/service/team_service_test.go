//go:build unit

package service

import (
	"context"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/domain"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/stretchr/testify/require"
)

// --- fake: UserRepository（仅实现 team_service 用到的两个方法，其余 panic） ---

type fakeTeamUserRepo struct {
	byID    map[int64]*User
	byEmail map[string]*User
}

func newFakeTeamUserRepo(users ...*User) *fakeTeamUserRepo {
	r := &fakeTeamUserRepo{byID: map[int64]*User{}, byEmail: map[string]*User{}}
	for _, u := range users {
		r.byID[u.ID] = u
		r.byEmail[u.Email] = u
	}
	return r
}

func (r *fakeTeamUserRepo) Create(context.Context, *User) error { panic("not implemented") }
func (r *fakeTeamUserRepo) GetByID(_ context.Context, id int64) (*User, error) {
	if u, ok := r.byID[id]; ok {
		cloned := *u
		return &cloned, nil
	}
	return nil, ErrUserNotFound
}
func (r *fakeTeamUserRepo) GetByIDIncludeDeleted(context.Context, int64) (*User, error) {
	panic("not implemented")
}
func (r *fakeTeamUserRepo) GetByEmail(_ context.Context, email string) (*User, error) {
	if u, ok := r.byEmail[email]; ok {
		cloned := *u
		return &cloned, nil
	}
	return nil, ErrUserNotFound
}
func (r *fakeTeamUserRepo) GetFirstAdmin(context.Context) (*User, error) { panic("not implemented") }
func (r *fakeTeamUserRepo) Update(context.Context, *User) error          { panic("not implemented") }
func (r *fakeTeamUserRepo) Delete(context.Context, int64) error          { panic("not implemented") }
func (r *fakeTeamUserRepo) GetUserAvatar(context.Context, int64) (*UserAvatar, error) {
	panic("not implemented")
}
func (r *fakeTeamUserRepo) UpsertUserAvatar(context.Context, int64, UpsertUserAvatarInput) (*UserAvatar, error) {
	panic("not implemented")
}
func (r *fakeTeamUserRepo) DeleteUserAvatar(context.Context, int64) error { panic("not implemented") }
func (r *fakeTeamUserRepo) List(context.Context, pagination.PaginationParams) ([]User, *pagination.PaginationResult, error) {
	panic("not implemented")
}
func (r *fakeTeamUserRepo) ListWithFilters(context.Context, pagination.PaginationParams, UserListFilters) ([]User, *pagination.PaginationResult, error) {
	panic("not implemented")
}
func (r *fakeTeamUserRepo) GetLatestUsedAtByUserIDs(context.Context, []int64) (map[int64]*time.Time, error) {
	panic("not implemented")
}
func (r *fakeTeamUserRepo) GetLatestUsedAtByUserID(context.Context, int64) (*time.Time, error) {
	panic("not implemented")
}
func (r *fakeTeamUserRepo) UpdateUserLastActiveAt(context.Context, int64, time.Time) error {
	panic("not implemented")
}
func (r *fakeTeamUserRepo) UpdateBalance(context.Context, int64, float64) error {
	panic("not implemented")
}
func (r *fakeTeamUserRepo) DeductBalance(context.Context, int64, float64) error {
	panic("not implemented")
}
func (r *fakeTeamUserRepo) UpdateConcurrency(context.Context, int64, int) error {
	panic("not implemented")
}
func (r *fakeTeamUserRepo) BatchSetConcurrency(context.Context, []int64, int) (int, error) {
	panic("not implemented")
}
func (r *fakeTeamUserRepo) BatchAddConcurrency(context.Context, []int64, int) (int, error) {
	panic("not implemented")
}
func (r *fakeTeamUserRepo) BatchUpdateLimits(context.Context, []int64, *int, *int) (int, error) {
	panic("not implemented")
}
func (r *fakeTeamUserRepo) ExistsByEmail(context.Context, string) (bool, error) {
	panic("not implemented")
}
func (r *fakeTeamUserRepo) GetByPhone(context.Context, string) (*User, error) {
	panic("not implemented")
}
func (r *fakeTeamUserRepo) ExistsByPhone(context.Context, string) (bool, error) {
	panic("not implemented")
}
func (r *fakeTeamUserRepo) RemoveGroupFromAllowedGroups(context.Context, int64) (int64, error) {
	panic("not implemented")
}
func (r *fakeTeamUserRepo) AddGroupToAllowedGroups(context.Context, int64, int64) error {
	panic("not implemented")
}
func (r *fakeTeamUserRepo) RemoveGroupFromUserAllowedGroups(context.Context, int64, int64) error {
	panic("not implemented")
}
func (r *fakeTeamUserRepo) ListUserAuthIdentities(context.Context, int64) ([]UserAuthIdentityRecord, error) {
	panic("not implemented")
}
func (r *fakeTeamUserRepo) UnbindUserAuthProvider(context.Context, int64, string) error {
	panic("not implemented")
}
func (r *fakeTeamUserRepo) UpdateTotpSecret(context.Context, int64, *string) error {
	panic("not implemented")
}
func (r *fakeTeamUserRepo) EnableTotp(context.Context, int64) error  { panic("not implemented") }
func (r *fakeTeamUserRepo) DisableTotp(context.Context, int64) error { panic("not implemented") }

// --- fake: TeamMemberRepository ---

type fakeTeamMemberRepo struct {
	members []TeamMember
	nextID  int64
	invRepo *fakeTeamInvitationRepo
}

func (r *fakeTeamMemberRepo) Create(_ context.Context, ownerUserID, memberUserID int64, role string) (*TeamMember, error) {
	r.nextID++
	m := TeamMember{ID: r.nextID, OwnerUserID: ownerUserID, MemberUserID: memberUserID, Role: role, Status: TeamMemberStatusActive, QuotaMode: domain.TeamQuotaModeAllocated, CreatedAt: time.Now()}
	r.members = append(r.members, m)
	return &m, nil
}

func (r *fakeTeamMemberRepo) AcceptInvitation(ctx context.Context, p AcceptInvitationParams) (*TeamMember, error) {
	for i := range r.members {
		if r.members[i].OwnerUserID == p.OwnerUserID && r.members[i].MemberUserID == p.MemberUserID && r.members[i].Status == TeamMemberStatusActive {
			if r.invRepo != nil {
				if err := r.invRepo.MarkAccepted(ctx, p.InvitationID, p.MemberUserID, p.AcceptedAt); err != nil {
					return nil, err
				}
			}
			r.members[i].Role = p.Role
			return &r.members[i], nil
		}
	}

	if p.MemberLimit > 0 {
		active := 0
		for _, m := range r.members {
			if m.OwnerUserID == p.OwnerUserID && m.Status == TeamMemberStatusActive {
				active++
			}
		}
		if active+1 >= p.MemberLimit {
			return nil, ErrTeamMemberLimitReached
		}
	}

	if r.invRepo != nil {
		if err := r.invRepo.MarkAccepted(ctx, p.InvitationID, p.MemberUserID, p.AcceptedAt); err != nil {
			return nil, err
		}
	}
	for i := range r.members {
		if r.members[i].OwnerUserID == p.OwnerUserID && r.members[i].MemberUserID == p.MemberUserID {
			r.members[i].Role = p.Role
			r.members[i].Status = TeamMemberStatusActive
			r.members[i].DepartmentID = p.DepartmentID
			r.members[i].QuotaMode = p.QuotaMode
			return &r.members[i], nil
		}
	}
	r.nextID++
	m := TeamMember{ID: r.nextID, OwnerUserID: p.OwnerUserID, MemberUserID: p.MemberUserID, Role: p.Role, Status: TeamMemberStatusActive, DepartmentID: p.DepartmentID, QuotaMode: p.QuotaMode, CreatedAt: time.Now()}
	r.members = append(r.members, m)
	return &m, nil
}

func (r *fakeTeamMemberRepo) FindActive(_ context.Context, ownerUserID, memberUserID int64) (*TeamMember, error) {
	for i := range r.members {
		m := r.members[i]
		if m.OwnerUserID == ownerUserID && m.MemberUserID == memberUserID && m.Status == TeamMemberStatusActive {
			return &m, nil
		}
	}
	return nil, nil
}

func (r *fakeTeamMemberRepo) ListActiveByOwner(_ context.Context, ownerUserID int64) ([]TeamMember, error) {
	var out []TeamMember
	for _, m := range r.members {
		if m.OwnerUserID == ownerUserID && m.Status == TeamMemberStatusActive {
			out = append(out, m)
		}
	}
	return out, nil
}

func (r *fakeTeamMemberRepo) ListActiveByMember(_ context.Context, memberUserID int64) ([]TeamMember, error) {
	var out []TeamMember
	for _, m := range r.members {
		if m.MemberUserID == memberUserID && m.Status == TeamMemberStatusActive {
			out = append(out, m)
		}
	}
	return out, nil
}

func (r *fakeTeamMemberRepo) Remove(_ context.Context, ownerUserID, memberUserID int64) (bool, error) {
	for i := range r.members {
		if r.members[i].OwnerUserID == ownerUserID && r.members[i].MemberUserID == memberUserID && r.members[i].Status == TeamMemberStatusActive {
			r.members[i].Status = TeamMemberStatusRemoved
			return true, nil
		}
	}
	return false, nil
}

func (r *fakeTeamMemberRepo) findActiveIdx(ownerUserID, memberUserID int64) int {
	for i := range r.members {
		if r.members[i].OwnerUserID == ownerUserID && r.members[i].MemberUserID == memberUserID && r.members[i].Status == TeamMemberStatusActive {
			return i
		}
	}
	return -1
}

func (r *fakeTeamMemberRepo) SetDepartment(_ context.Context, ownerUserID, memberUserID int64, departmentID *int64) (bool, error) {
	i := r.findActiveIdx(ownerUserID, memberUserID)
	if i < 0 {
		return false, nil
	}
	r.members[i].DepartmentID = departmentID
	return true, nil
}

func (r *fakeTeamMemberRepo) SetQuotaSettings(_ context.Context, ownerUserID, memberUserID int64, quotaMode string, threshold, target *float64) (bool, error) {
	i := r.findActiveIdx(ownerUserID, memberUserID)
	if i < 0 {
		return false, nil
	}
	r.members[i].QuotaMode = quotaMode
	r.members[i].AutoTopupThresholdUSD = threshold
	r.members[i].AutoTopupTargetUSD = target
	return true, nil
}

func (r *fakeTeamMemberRepo) SetRole(_ context.Context, ownerUserID, memberUserID int64, role string) (bool, error) {
	i := r.findActiveIdx(ownerUserID, memberUserID)
	if i < 0 {
		return false, nil
	}
	r.members[i].Role = role
	return true, nil
}

func (r *fakeTeamMemberRepo) ListTopupCandidates(context.Context, int) ([]TeamTopupCandidate, error) {
	return nil, nil
}

// --- fake: TeamInvitationRepository ---

type fakeTeamInvitationRepo struct {
	invitations []TeamInvitation
	nextID      int64
}

func (r *fakeTeamInvitationRepo) Create(_ context.Context, in TeamInvitationCreateInput) (*TeamInvitation, error) {
	r.nextID++
	now := time.Now()
	inv := TeamInvitation{
		ID: r.nextID, OwnerUserID: in.OwnerUserID, InvitedEmail: in.InvitedEmail, Token: in.Token,
		Status: TeamInvitationStatusPending, ExpiresAt: in.ExpiresAt, LastSentAt: now,
		DepartmentID: in.DepartmentID, Role: in.Role, QuotaMode: in.QuotaMode, InitialGrantUSD: in.InitialGrantUSD,
		CreatedAt: now,
	}
	r.invitations = append(r.invitations, inv)
	return &inv, nil
}

func (r *fakeTeamInvitationRepo) FindPendingByToken(_ context.Context, token string) (*TeamInvitation, error) {
	for i := range r.invitations {
		inv := r.invitations[i]
		if inv.Token == token && inv.Status == TeamInvitationStatusPending {
			return &inv, nil
		}
	}
	return nil, nil
}

func (r *fakeTeamInvitationRepo) FindPendingByID(_ context.Context, id, ownerUserID int64) (*TeamInvitation, error) {
	for i := range r.invitations {
		inv := r.invitations[i]
		if inv.ID == id && inv.OwnerUserID == ownerUserID && inv.Status == TeamInvitationStatusPending {
			return &inv, nil
		}
	}
	return nil, nil
}

func (r *fakeTeamInvitationRepo) FindPendingByOwnerAndEmail(_ context.Context, ownerUserID int64, invitedEmail string, now time.Time) (*TeamInvitation, error) {
	for i := len(r.invitations) - 1; i >= 0; i-- {
		inv := r.invitations[i]
		if inv.OwnerUserID == ownerUserID && inv.InvitedEmail == invitedEmail && inv.Status == TeamInvitationStatusPending && inv.ExpiresAt.After(now) {
			return &inv, nil
		}
	}
	return nil, nil
}

func (r *fakeTeamInvitationRepo) ListPendingByOwner(_ context.Context, ownerUserID int64) ([]TeamInvitation, error) {
	var out []TeamInvitation
	for _, inv := range r.invitations {
		if inv.OwnerUserID == ownerUserID && inv.Status == TeamInvitationStatusPending {
			out = append(out, inv)
		}
	}
	return out, nil
}

func (r *fakeTeamInvitationRepo) ListPendingByInvitedEmail(_ context.Context, invitedEmail string, now time.Time) ([]TeamInvitation, error) {
	var out []TeamInvitation
	for _, inv := range r.invitations {
		if inv.InvitedEmail == invitedEmail && inv.Status == TeamInvitationStatusPending && inv.ExpiresAt.After(now) {
			out = append(out, inv)
		}
	}
	return out, nil
}

func (r *fakeTeamInvitationRepo) MarkAccepted(_ context.Context, id, acceptedByUserID int64, acceptedAt time.Time) error {
	for i := range r.invitations {
		if r.invitations[i].ID == id {
			r.invitations[i].Status = TeamInvitationStatusAccepted
			r.invitations[i].AcceptedByUserID = &acceptedByUserID
			r.invitations[i].AcceptedAt = &acceptedAt
		}
	}
	return nil
}

func (r *fakeTeamInvitationRepo) TryMarkSent(_ context.Context, id int64, now time.Time, minInterval time.Duration) (bool, error) {
	for i := range r.invitations {
		if r.invitations[i].ID == id {
			if r.invitations[i].Status != TeamInvitationStatusPending || r.invitations[i].LastSentAt.After(now.Add(-minInterval)) {
				return false, nil
			}
			r.invitations[i].LastSentAt = now
			return true, nil
		}
	}
	return false, nil
}

func (r *fakeTeamInvitationRepo) Revoke(_ context.Context, id, ownerUserID int64) (bool, error) {
	for i := range r.invitations {
		if r.invitations[i].ID == id && r.invitations[i].OwnerUserID == ownerUserID && r.invitations[i].Status == TeamInvitationStatusPending {
			r.invitations[i].Status = TeamInvitationStatusRevoked
			return true, nil
		}
	}
	return false, nil
}

// --- fake: EnterpriseProfileRepository ---

type fakeEnterpriseProfileRepo struct {
	profiles map[int64]*EnterpriseProfile
}

func newFakeEnterpriseProfileRepo(upgradedOwners ...int64) *fakeEnterpriseProfileRepo {
	r := &fakeEnterpriseProfileRepo{profiles: map[int64]*EnterpriseProfile{}}
	for _, id := range upgradedOwners {
		r.profiles[id] = &EnterpriseProfile{UserID: id, CompanyName: "Acme"}
	}
	return r
}

func (r *fakeEnterpriseProfileRepo) Create(_ context.Context, userID int64, companyName, contactName, contactPhone, industry string) (*EnterpriseProfile, error) {
	if _, ok := r.profiles[userID]; ok {
		return nil, ErrEnterpriseAlreadyUpgraded
	}
	p := &EnterpriseProfile{UserID: userID, CompanyName: companyName, ContactName: contactName, ContactPhone: contactPhone, Industry: industry, CreatedAt: time.Now()}
	r.profiles[userID] = p
	return p, nil
}

func (r *fakeEnterpriseProfileRepo) GetByUserID(_ context.Context, userID int64) (*EnterpriseProfile, error) {
	return r.profiles[userID], nil
}

// --- fake: TeamDepartmentRepository ---

type fakeTeamDepartmentRepo struct {
	departments []TeamDepartment
	nextID      int64
}

func (r *fakeTeamDepartmentRepo) Create(_ context.Context, ownerUserID int64, name string, displayOrder int) (*TeamDepartment, error) {
	for _, d := range r.departments {
		if d.OwnerUserID == ownerUserID && d.Name == name {
			return nil, ErrTeamDepartmentNameTaken
		}
	}
	r.nextID++
	d := TeamDepartment{ID: r.nextID, OwnerUserID: ownerUserID, Name: name, DisplayOrder: displayOrder, CreatedAt: time.Now()}
	r.departments = append(r.departments, d)
	return &d, nil
}

func (r *fakeTeamDepartmentRepo) List(_ context.Context, ownerUserID int64) ([]TeamDepartment, error) {
	var out []TeamDepartment
	for _, d := range r.departments {
		if d.OwnerUserID == ownerUserID {
			out = append(out, d)
		}
	}
	return out, nil
}

func (r *fakeTeamDepartmentRepo) Exists(_ context.Context, id, ownerUserID int64) (bool, error) {
	for _, d := range r.departments {
		if d.ID == id && d.OwnerUserID == ownerUserID {
			return true, nil
		}
	}
	return false, nil
}

func (r *fakeTeamDepartmentRepo) Update(_ context.Context, id, ownerUserID int64, name string, displayOrder int) (bool, error) {
	for i := range r.departments {
		if r.departments[i].ID == id && r.departments[i].OwnerUserID == ownerUserID {
			r.departments[i].Name = name
			r.departments[i].DisplayOrder = displayOrder
			return true, nil
		}
	}
	return false, nil
}

func (r *fakeTeamDepartmentRepo) Delete(_ context.Context, id, ownerUserID int64) (bool, error) {
	for i := range r.departments {
		if r.departments[i].ID == id && r.departments[i].OwnerUserID == ownerUserID {
			r.departments = append(r.departments[:i], r.departments[i+1:]...)
			return true, nil
		}
	}
	return false, nil
}

// --- fake: TeamFundRepository ---

type fakeTeamFundRepo struct {
	balances  map[int64]*float64 // 指向 fakeTeamUserRepo 里 User.Balance，便于同步观察
	members   *fakeTeamMemberRepo
	transfers []TeamFundTransfer
	nextID    int64
}

func (r *fakeTeamFundRepo) Transfer(_ context.Context, ownerUserID, memberUserID int64, direction string, amount float64, operatorUserID int64, note string) (*TeamFundTransfer, error) {
	if amount <= 0 {
		return nil, ErrTeamInvalidTransferAmount
	}
	idx := r.members.findActiveIdx(ownerUserID, memberUserID)
	if idx < 0 {
		return nil, ErrTeamMemberNotFound
	}
	ownerBal := r.balances[ownerUserID]
	memberBal := r.balances[memberUserID]

	switch direction {
	case domain.TeamFundDirectionGrant, domain.TeamFundDirectionAutoTopup:
		if *ownerBal < amount {
			return nil, ErrTeamInsufficientBalance
		}
		*ownerBal -= amount
		*memberBal += amount
		r.members.members[idx].GrantedNetUSD += amount
	case domain.TeamFundDirectionReclaim:
		limit := r.members.members[idx].GrantedNetUSD
		if *memberBal < limit {
			limit = *memberBal
		}
		if amount > limit {
			return nil, ErrTeamReclaimExceedsLimit
		}
		*memberBal -= amount
		*ownerBal += amount
		r.members.members[idx].GrantedNetUSD -= amount
	default:
		return nil, ErrTeamInvalidTransferDirection
	}

	r.nextID++
	t := TeamFundTransfer{ID: r.nextID, OwnerUserID: ownerUserID, MemberUserID: memberUserID, Direction: direction, Amount: amount, OperatorUserID: operatorUserID, Note: note, CreatedAt: time.Now()}
	r.transfers = append(r.transfers, t)
	return &t, nil
}

func (r *fakeTeamFundRepo) ListByOwner(_ context.Context, ownerUserID int64, offset, limit int) ([]TeamFundTransfer, int, error) {
	var all []TeamFundTransfer
	for _, t := range r.transfers {
		if t.OwnerUserID == ownerUserID {
			all = append(all, t)
		}
	}
	total := len(all)
	if offset >= total {
		return nil, total, nil
	}
	end := offset + limit
	if end > total {
		end = total
	}
	return all[offset:end], total, nil
}

// --- test helpers ---

func newTeamServiceForTest(userRepo UserRepository, memberRepo TeamMemberRepository, invRepo TeamInvitationRepository) *TeamService {
	return newTeamServiceForTestWithEnterprise(userRepo, memberRepo, invRepo, nil)
}

func newTeamServiceForTestWithEnterprise(userRepo UserRepository, memberRepo TeamMemberRepository, invRepo TeamInvitationRepository, enterpriseRepo EnterpriseProfileRepository) *TeamService {
	if member, ok := memberRepo.(*fakeTeamMemberRepo); ok {
		if invitation, ok := invRepo.(*fakeTeamInvitationRepo); ok {
			member.invRepo = invitation
		}
	}
	return NewTeamService(memberRepo, invRepo, nil, userRepo, enterpriseRepo, nil, nil, nil, nil)
}

const testFrontendURL = "https://panel.example.com"

func defaultInvite(email string) TeamInviteInput {
	return TeamInviteInput{Email: email, Role: domain.TeamMemberRoleMember, QuotaMode: domain.TeamQuotaModeAllocated}
}

// --- InviteMember ---

func TestTeamService_InviteMember_RejectsNonMember(t *testing.T) {
	owner := &User{ID: 1, Email: "owner@example.com"}
	userRepo := newFakeTeamUserRepo(owner)
	svc := newTeamServiceForTestWithEnterprise(userRepo, &fakeTeamMemberRepo{}, &fakeTeamInvitationRepo{}, newFakeEnterpriseProfileRepo(1))

	_, err := svc.InviteMember(context.Background(), owner.ID, 999, defaultInvite("admin@example.com"), testFrontendURL)
	require.ErrorIs(t, err, ErrInsufficientPerms)
}

func TestTeamService_InviteMember_RequiresEnterpriseUpgrade(t *testing.T) {
	owner := &User{ID: 1, Email: "owner@example.com"}
	userRepo := newFakeTeamUserRepo(owner)
	svc := newTeamServiceForTestWithEnterprise(userRepo, &fakeTeamMemberRepo{}, &fakeTeamInvitationRepo{}, newFakeEnterpriseProfileRepo())

	_, err := svc.InviteMember(context.Background(), owner.ID, owner.ID, defaultInvite("admin@example.com"), testFrontendURL)
	require.ErrorIs(t, err, ErrEnterpriseRequired)
}

func TestTeamService_InviteMember_RejectsSelf(t *testing.T) {
	owner := &User{ID: 1, Email: "owner@example.com"}
	userRepo := newFakeTeamUserRepo(owner)
	svc := newTeamServiceForTestWithEnterprise(userRepo, &fakeTeamMemberRepo{}, &fakeTeamInvitationRepo{}, newFakeEnterpriseProfileRepo(1))

	_, err := svc.InviteMember(context.Background(), owner.ID, owner.ID, defaultInvite("Owner@Example.com"), testFrontendURL)
	require.ErrorIs(t, err, ErrTeamCannotInviteSelf)
}

func TestTeamService_InviteMember_RejectsExistingActiveMember(t *testing.T) {
	owner := &User{ID: 1, Email: "owner@example.com"}
	admin := &User{ID: 2, Email: "admin@example.com"}
	userRepo := newFakeTeamUserRepo(owner, admin)
	memberRepo := &fakeTeamMemberRepo{}
	_, err := memberRepo.Create(context.Background(), owner.ID, admin.ID, TeamMemberRoleAdmin)
	require.NoError(t, err)
	svc := newTeamServiceForTestWithEnterprise(userRepo, memberRepo, &fakeTeamInvitationRepo{}, newFakeEnterpriseProfileRepo(1))

	_, err = svc.InviteMember(context.Background(), owner.ID, owner.ID, defaultInvite(admin.Email), testFrontendURL)
	require.ErrorIs(t, err, ErrTeamAlreadyMember)
}

func TestTeamService_InviteMember_Success(t *testing.T) {
	owner := &User{ID: 1, Email: "owner@example.com"}
	userRepo := newFakeTeamUserRepo(owner)
	svc := newTeamServiceForTestWithEnterprise(userRepo, &fakeTeamMemberRepo{}, &fakeTeamInvitationRepo{}, newFakeEnterpriseProfileRepo(1))

	inv, err := svc.InviteMember(context.Background(), owner.ID, owner.ID, defaultInvite("New.Admin@Example.com"), testFrontendURL)
	require.NoError(t, err)
	require.Equal(t, "new.admin@example.com", inv.InvitedEmail)
	require.Equal(t, TeamInvitationStatusPending, inv.Status)
	require.Equal(t, domain.TeamMemberRoleMember, inv.Role)
	require.NotEmpty(t, inv.Token)
	require.True(t, inv.ExpiresAt.After(time.Now()))
}

func TestTeamService_InviteMember_AdminCannotInviteAsAdmin(t *testing.T) {
	owner := &User{ID: 1, Email: "owner@example.com"}
	admin := &User{ID: 2, Email: "admin@example.com"}
	userRepo := newFakeTeamUserRepo(owner, admin)
	memberRepo := &fakeTeamMemberRepo{}
	_, err := memberRepo.Create(context.Background(), owner.ID, admin.ID, domain.TeamMemberRoleAdmin)
	require.NoError(t, err)
	svc := newTeamServiceForTestWithEnterprise(userRepo, memberRepo, &fakeTeamInvitationRepo{}, newFakeEnterpriseProfileRepo(1))

	in := defaultInvite("new@example.com")
	in.Role = domain.TeamMemberRoleAdmin
	_, err = svc.InviteMember(context.Background(), owner.ID, admin.ID, in, testFrontendURL)
	require.ErrorIs(t, err, ErrTeamOwnerOnly)
}

func TestTeamService_InviteMember_OwnerCanInviteAsAdmin(t *testing.T) {
	owner := &User{ID: 1, Email: "owner@example.com"}
	userRepo := newFakeTeamUserRepo(owner)
	svc := newTeamServiceForTestWithEnterprise(userRepo, &fakeTeamMemberRepo{}, &fakeTeamInvitationRepo{}, newFakeEnterpriseProfileRepo(1))

	in := defaultInvite("new-admin@example.com")
	in.Role = domain.TeamMemberRoleAdmin
	inv, err := svc.InviteMember(context.Background(), owner.ID, owner.ID, in, testFrontendURL)
	require.NoError(t, err)
	require.Equal(t, domain.TeamMemberRoleAdmin, inv.Role)
}

func TestTeamService_InviteMember_RejectsUnknownDepartment(t *testing.T) {
	owner := &User{ID: 1, Email: "owner@example.com"}
	userRepo := newFakeTeamUserRepo(owner)
	svc := NewTeamService(&fakeTeamMemberRepo{}, &fakeTeamInvitationRepo{}, nil, userRepo, newFakeEnterpriseProfileRepo(1), &fakeTeamDepartmentRepo{}, nil, nil, nil)

	missing := int64(999)
	in := defaultInvite("new@example.com")
	in.DepartmentID = &missing
	_, err := svc.InviteMember(context.Background(), owner.ID, owner.ID, in, testFrontendURL)
	require.ErrorIs(t, err, ErrTeamDepartmentNotFound)
}

func TestTeamService_InviteMember_ImmediateReinviteIsRateLimited(t *testing.T) {
	owner := &User{ID: 1, Email: "owner@example.com"}
	userRepo := newFakeTeamUserRepo(owner)
	invRepo := &fakeTeamInvitationRepo{}
	svc := newTeamServiceForTestWithEnterprise(userRepo, &fakeTeamMemberRepo{}, invRepo, newFakeEnterpriseProfileRepo(1))

	first, err := svc.InviteMember(context.Background(), owner.ID, owner.ID, defaultInvite("admin@example.com"), testFrontendURL)
	require.NoError(t, err)

	_, err = svc.InviteMember(context.Background(), owner.ID, owner.ID, defaultInvite("ADMIN@example.com"), testFrontendURL)
	require.ErrorIs(t, err, ErrTeamInvitationResendTooSoon)
	require.Len(t, invRepo.invitations, 1)
	require.Equal(t, first.ID, invRepo.invitations[0].ID)
}

func TestTeamService_InviteMember_ReusesExistingPendingInvitationAfterInterval(t *testing.T) {
	owner := &User{ID: 1, Email: "owner@example.com"}
	userRepo := newFakeTeamUserRepo(owner)
	invRepo := &fakeTeamInvitationRepo{}
	svc := newTeamServiceForTestWithEnterprise(userRepo, &fakeTeamMemberRepo{}, invRepo, newFakeEnterpriseProfileRepo(1))

	first, err := svc.InviteMember(context.Background(), owner.ID, owner.ID, defaultInvite("admin@example.com"), testFrontendURL)
	require.NoError(t, err)
	invRepo.invitations[0].LastSentAt = time.Now().Add(-teamInvitationResendInterval - time.Second)

	second, err := svc.InviteMember(context.Background(), owner.ID, owner.ID, defaultInvite("ADMIN@example.com"), testFrontendURL)
	require.NoError(t, err)
	require.Equal(t, first.ID, second.ID)
	require.Len(t, invRepo.invitations, 1)
}

func TestTeamService_InviteMember_RejectsWhenMemberLimitReached(t *testing.T) {
	owner := &User{ID: 1, Email: "owner@example.com"}
	userRepo := newFakeTeamUserRepo(owner)
	memberRepo := &fakeTeamMemberRepo{}
	for i := int64(2); i < 2+teamMemberLimit-1; i++ {
		_, err := memberRepo.Create(context.Background(), owner.ID, i, domain.TeamMemberRoleMember)
		require.NoError(t, err)
	}
	svc := newTeamServiceForTestWithEnterprise(userRepo, memberRepo, &fakeTeamInvitationRepo{}, newFakeEnterpriseProfileRepo(1))

	_, err := svc.InviteMember(context.Background(), owner.ID, owner.ID, defaultInvite("newcomer@example.com"), testFrontendURL)
	require.ErrorIs(t, err, ErrTeamMemberLimitReached)
}

// --- AcceptInvitation ---

// --- ListReceivedInvitations ---

func TestTeamService_ListReceivedInvitations_ReturnsPendingForMyEmail(t *testing.T) {
	owner := &User{ID: 1, Email: "owner@example.com"}
	invitee := &User{ID: 2, Email: "Staff@Example.com"}
	userRepo := newFakeTeamUserRepo(owner, invitee)
	invRepo := &fakeTeamInvitationRepo{}
	svc := newTeamServiceForTestWithEnterprise(userRepo, &fakeTeamMemberRepo{}, invRepo, newFakeEnterpriseProfileRepo(1))

	inv, err := svc.InviteMember(context.Background(), owner.ID, owner.ID, defaultInvite(invitee.Email), testFrontendURL)
	require.NoError(t, err)

	received, err := svc.ListReceivedInvitations(context.Background(), invitee.ID)
	require.NoError(t, err)
	require.Len(t, received, 1)
	require.Equal(t, inv.ID, received[0].ID)
	require.Equal(t, owner.ID, received[0].OwnerUserID)
	require.Equal(t, owner.Email, received[0].OwnerEmail)
	require.Equal(t, inv.Token, received[0].Token)

	// 用返回的 token 走既有接受流程应当成功。
	member, err := svc.AcceptInvitation(context.Background(), received[0].Token, invitee.ID)
	require.NoError(t, err)
	require.Equal(t, owner.ID, member.OwnerUserID)

	// 接受后不再出现在"我收到的邀请"里。
	received, err = svc.ListReceivedInvitations(context.Background(), invitee.ID)
	require.NoError(t, err)
	require.Empty(t, received)
}

func TestTeamService_ListReceivedInvitations_ExcludesExpiredAndOthers(t *testing.T) {
	owner := &User{ID: 1, Email: "owner@example.com"}
	invitee := &User{ID: 2, Email: "staff@example.com"}
	userRepo := newFakeTeamUserRepo(owner, invitee)
	invRepo := &fakeTeamInvitationRepo{}
	invRepo.invitations = append(invRepo.invitations,
		TeamInvitation{ID: 101, OwnerUserID: owner.ID, InvitedEmail: "staff@example.com", Token: "expired-token", Status: TeamInvitationStatusPending, ExpiresAt: time.Now().Add(-time.Hour)},
		TeamInvitation{ID: 102, OwnerUserID: owner.ID, InvitedEmail: "other@example.com", Token: "other-token", Status: TeamInvitationStatusPending, ExpiresAt: time.Now().Add(time.Hour)},
	)
	svc := newTeamServiceForTest(userRepo, &fakeTeamMemberRepo{}, invRepo)

	received, err := svc.ListReceivedInvitations(context.Background(), invitee.ID)
	require.NoError(t, err)
	require.Empty(t, received)
}

func TestTeamService_AcceptInvitation_NotFound(t *testing.T) {
	userRepo := newFakeTeamUserRepo(&User{ID: 1, Email: "owner@example.com"})
	svc := newTeamServiceForTest(userRepo, &fakeTeamMemberRepo{}, &fakeTeamInvitationRepo{})

	_, err := svc.AcceptInvitation(context.Background(), "nonexistent-token", 2)
	require.ErrorIs(t, err, ErrTeamInvitationNotFound)
}

func TestTeamService_AcceptInvitation_Expired(t *testing.T) {
	owner := &User{ID: 1, Email: "owner@example.com"}
	invitee := &User{ID: 2, Email: "admin@example.com"}
	userRepo := newFakeTeamUserRepo(owner, invitee)
	invRepo := &fakeTeamInvitationRepo{}
	_, err := invRepo.Create(context.Background(), TeamInvitationCreateInput{OwnerUserID: owner.ID, InvitedEmail: invitee.Email, Token: "tok-expired", ExpiresAt: time.Now().Add(-time.Hour), Role: domain.TeamMemberRoleMember, QuotaMode: domain.TeamQuotaModeAllocated})
	require.NoError(t, err)
	svc := newTeamServiceForTest(userRepo, &fakeTeamMemberRepo{}, invRepo)

	_, err = svc.AcceptInvitation(context.Background(), "tok-expired", invitee.ID)
	require.ErrorIs(t, err, ErrTeamInvitationExpired)
}

func TestTeamService_AcceptInvitation_EmailMismatch(t *testing.T) {
	owner := &User{ID: 1, Email: "owner@example.com"}
	invitee := &User{ID: 2, Email: "someone-else@example.com"}
	userRepo := newFakeTeamUserRepo(owner, invitee)
	invRepo := &fakeTeamInvitationRepo{}
	_, err := invRepo.Create(context.Background(), TeamInvitationCreateInput{OwnerUserID: owner.ID, InvitedEmail: "admin@example.com", Token: "tok-mismatch", ExpiresAt: time.Now().Add(time.Hour), Role: domain.TeamMemberRoleMember, QuotaMode: domain.TeamQuotaModeAllocated})
	require.NoError(t, err)
	svc := newTeamServiceForTest(userRepo, &fakeTeamMemberRepo{}, invRepo)

	_, err = svc.AcceptInvitation(context.Background(), "tok-mismatch", invitee.ID)
	require.ErrorIs(t, err, ErrTeamInvitationEmailMismatch)
}

func TestTeamService_AcceptInvitation_Success(t *testing.T) {
	owner := &User{ID: 1, Email: "owner@example.com"}
	invitee := &User{ID: 2, Email: "admin@example.com"}
	userRepo := newFakeTeamUserRepo(owner, invitee)
	invRepo := &fakeTeamInvitationRepo{}
	_, err := invRepo.Create(context.Background(), TeamInvitationCreateInput{OwnerUserID: owner.ID, InvitedEmail: invitee.Email, Token: "tok-ok", ExpiresAt: time.Now().Add(time.Hour), Role: domain.TeamMemberRoleMember, QuotaMode: domain.TeamQuotaModeAllocated})
	require.NoError(t, err)
	memberRepo := &fakeTeamMemberRepo{}
	svc := newTeamServiceForTest(userRepo, memberRepo, invRepo)

	member, err := svc.AcceptInvitation(context.Background(), "tok-ok", invitee.ID)
	require.NoError(t, err)
	require.Equal(t, owner.ID, member.OwnerUserID)
	require.Equal(t, invitee.ID, member.MemberUserID)
	require.Equal(t, domain.TeamMemberRoleMember, member.Role)
	require.Len(t, memberRepo.members, 1)

	// token 消费后（已标记 accepted）不可再次使用；FindActive 幂等分支只保护并发双击场景。
	_, err = svc.AcceptInvitation(context.Background(), "tok-ok", invitee.ID)
	require.ErrorIs(t, err, ErrTeamInvitationNotFound)
}

func TestTeamService_AcceptInvitation_ConcurrentDoubleClickIsIdempotent(t *testing.T) {
	owner := &User{ID: 1, Email: "owner@example.com"}
	invitee := &User{ID: 2, Email: "admin@example.com"}
	userRepo := newFakeTeamUserRepo(owner, invitee)
	invRepo := &fakeTeamInvitationRepo{}
	_, err := invRepo.Create(context.Background(), TeamInvitationCreateInput{OwnerUserID: owner.ID, InvitedEmail: invitee.Email, Token: "tok-race", ExpiresAt: time.Now().Add(time.Hour), Role: domain.TeamMemberRoleMember, QuotaMode: domain.TeamQuotaModeAllocated})
	require.NoError(t, err)
	memberRepo := &fakeTeamMemberRepo{}
	// 模拟并发双击：成员关系已被另一个请求创建，但邀请状态仍是 pending（尚未来得及 MarkAccepted）。
	_, err = memberRepo.Create(context.Background(), owner.ID, invitee.ID, domain.TeamMemberRoleMember)
	require.NoError(t, err)
	svc := newTeamServiceForTest(userRepo, memberRepo, invRepo)

	member, err := svc.AcceptInvitation(context.Background(), "tok-race", invitee.ID)
	require.NoError(t, err)
	require.Equal(t, owner.ID, member.OwnerUserID)
	require.Len(t, memberRepo.members, 1)
}

func TestTeamService_AcceptInvitation_ReactivatesRemovedMember(t *testing.T) {
	owner := &User{ID: 1, Email: "owner@example.com"}
	invitee := &User{ID: 2, Email: "admin@example.com"}
	userRepo := newFakeTeamUserRepo(owner, invitee)
	invRepo := &fakeTeamInvitationRepo{}
	_, err := invRepo.Create(context.Background(), TeamInvitationCreateInput{OwnerUserID: owner.ID, InvitedEmail: invitee.Email, Token: "tok-rejoin", ExpiresAt: time.Now().Add(time.Hour), Role: domain.TeamMemberRoleMember, QuotaMode: domain.TeamQuotaModeAllocated})
	require.NoError(t, err)
	memberRepo := &fakeTeamMemberRepo{}
	_, err = memberRepo.Create(context.Background(), owner.ID, invitee.ID, domain.TeamMemberRoleMember)
	require.NoError(t, err)
	removed, err := memberRepo.Remove(context.Background(), owner.ID, invitee.ID)
	require.NoError(t, err)
	require.True(t, removed)
	svc := newTeamServiceForTest(userRepo, memberRepo, invRepo)

	member, err := svc.AcceptInvitation(context.Background(), "tok-rejoin", invitee.ID)
	require.NoError(t, err)
	require.Equal(t, TeamMemberStatusActive, member.Status)
	require.Len(t, memberRepo.members, 1)
}

func TestTeamService_AcceptInvitation_RejectsWhenMemberLimitReached(t *testing.T) {
	owner := &User{ID: 1, Email: "owner@example.com"}
	invitee := &User{ID: 1000, Email: "admin@example.com"}
	userRepo := newFakeTeamUserRepo(owner, invitee)
	memberRepo := &fakeTeamMemberRepo{}
	for i := int64(2); i < 2+teamMemberLimit-1; i++ {
		_, err := memberRepo.Create(context.Background(), owner.ID, i, domain.TeamMemberRoleMember)
		require.NoError(t, err)
	}
	invRepo := &fakeTeamInvitationRepo{}
	_, err := invRepo.Create(context.Background(), TeamInvitationCreateInput{OwnerUserID: owner.ID, InvitedEmail: invitee.Email, Token: "tok-full", ExpiresAt: time.Now().Add(time.Hour), Role: domain.TeamMemberRoleMember, QuotaMode: domain.TeamQuotaModeAllocated})
	require.NoError(t, err)
	svc := newTeamServiceForTest(userRepo, memberRepo, invRepo)

	_, err = svc.AcceptInvitation(context.Background(), "tok-full", invitee.ID)
	require.ErrorIs(t, err, ErrTeamMemberLimitReached)
}

func TestTeamService_AcceptInvitation_CopiesDepartmentRoleQuotaModeFromInvitation(t *testing.T) {
	owner := &User{ID: 1, Email: "owner@example.com"}
	invitee := &User{ID: 2, Email: "admin@example.com"}
	userRepo := newFakeTeamUserRepo(owner, invitee)
	invRepo := &fakeTeamInvitationRepo{}
	dept := int64(42)
	_, err := invRepo.Create(context.Background(), TeamInvitationCreateInput{
		OwnerUserID: owner.ID, InvitedEmail: invitee.Email, Token: "tok-dept",
		ExpiresAt: time.Now().Add(time.Hour), DepartmentID: &dept,
		Role: domain.TeamMemberRoleAdmin, QuotaMode: domain.TeamQuotaModeShared,
	})
	require.NoError(t, err)
	memberRepo := &fakeTeamMemberRepo{}
	svc := newTeamServiceForTest(userRepo, memberRepo, invRepo)

	member, err := svc.AcceptInvitation(context.Background(), "tok-dept", invitee.ID)
	require.NoError(t, err)
	require.Equal(t, domain.TeamMemberRoleAdmin, member.Role)
	require.NotNil(t, member.DepartmentID)
	require.Equal(t, dept, *member.DepartmentID)
	require.Equal(t, domain.TeamQuotaModeShared, member.QuotaMode)
}

// --- ResendInvitation / RevokeInvitation ---

func TestTeamService_ResendInvitation_RejectsNonMember(t *testing.T) {
	owner := &User{ID: 1}
	userRepo := newFakeTeamUserRepo(owner)
	invRepo := &fakeTeamInvitationRepo{}
	inv, err := invRepo.Create(context.Background(), TeamInvitationCreateInput{OwnerUserID: owner.ID, InvitedEmail: "admin@example.com", Token: "tok-resend", ExpiresAt: time.Now().Add(time.Hour), Role: domain.TeamMemberRoleMember, QuotaMode: domain.TeamQuotaModeAllocated})
	require.NoError(t, err)
	svc := newTeamServiceForTest(userRepo, &fakeTeamMemberRepo{}, invRepo)

	err = svc.ResendInvitation(context.Background(), owner.ID, 999, inv.ID, testFrontendURL)
	require.ErrorIs(t, err, ErrInsufficientPerms)
}

func TestTeamService_ResendInvitation_RateLimited(t *testing.T) {
	owner := &User{ID: 1, Email: "owner@example.com"}
	userRepo := newFakeTeamUserRepo(owner)
	invRepo := &fakeTeamInvitationRepo{}
	inv, err := invRepo.Create(context.Background(), TeamInvitationCreateInput{OwnerUserID: owner.ID, InvitedEmail: "admin@example.com", Token: "tok-throttle", ExpiresAt: time.Now().Add(time.Hour), Role: domain.TeamMemberRoleMember, QuotaMode: domain.TeamQuotaModeAllocated})
	require.NoError(t, err)
	invRepo.invitations[0].LastSentAt = time.Now()
	svc := newTeamServiceForTest(userRepo, &fakeTeamMemberRepo{}, invRepo)

	err = svc.ResendInvitation(context.Background(), owner.ID, owner.ID, inv.ID, testFrontendURL)
	require.ErrorIs(t, err, ErrTeamInvitationResendTooSoon)
}

func TestTeamService_ResendInvitation_AllowedAfterInterval(t *testing.T) {
	owner := &User{ID: 1, Email: "owner@example.com"}
	userRepo := newFakeTeamUserRepo(owner)
	invRepo := &fakeTeamInvitationRepo{}
	inv, err := invRepo.Create(context.Background(), TeamInvitationCreateInput{OwnerUserID: owner.ID, InvitedEmail: "admin@example.com", Token: "tok-throttle-ok", ExpiresAt: time.Now().Add(time.Hour), Role: domain.TeamMemberRoleMember, QuotaMode: domain.TeamQuotaModeAllocated})
	require.NoError(t, err)
	invRepo.invitations[0].LastSentAt = time.Now().Add(-teamInvitationResendInterval - time.Second)
	svc := newTeamServiceForTest(userRepo, &fakeTeamMemberRepo{}, invRepo)

	err = svc.ResendInvitation(context.Background(), owner.ID, owner.ID, inv.ID, testFrontendURL)
	require.NoError(t, err)
	require.WithinDuration(t, time.Now(), invRepo.invitations[0].LastSentAt, time.Second)
}

func TestTeamService_RevokeInvitation_NotFoundWhenAlreadyHandled(t *testing.T) {
	owner := &User{ID: 1}
	userRepo := newFakeTeamUserRepo(owner)
	svc := newTeamServiceForTest(userRepo, &fakeTeamMemberRepo{}, &fakeTeamInvitationRepo{})

	err := svc.RevokeInvitation(context.Background(), owner.ID, owner.ID, 12345)
	require.ErrorIs(t, err, ErrTeamInvitationNotFound)
}

func TestTeamService_RevokeInvitation_RejectsNonMember(t *testing.T) {
	owner := &User{ID: 1}
	userRepo := newFakeTeamUserRepo(owner)
	invRepo := &fakeTeamInvitationRepo{}
	inv, err := invRepo.Create(context.Background(), TeamInvitationCreateInput{OwnerUserID: owner.ID, InvitedEmail: "admin@example.com", Token: "tok-revoke", ExpiresAt: time.Now().Add(time.Hour), Role: domain.TeamMemberRoleMember, QuotaMode: domain.TeamQuotaModeAllocated})
	require.NoError(t, err)
	svc := newTeamServiceForTest(userRepo, &fakeTeamMemberRepo{}, invRepo)

	err = svc.RevokeInvitation(context.Background(), owner.ID, 999, inv.ID)
	require.ErrorIs(t, err, ErrInsufficientPerms)
	require.Equal(t, TeamInvitationStatusPending, invRepo.invitations[0].Status)
}

// admin（非 owner）可发起邀请管理操作，验证 authorizeTeamManager 对 active admin 放行。
func TestTeamService_RevokeInvitation_AllowsActiveAdmin(t *testing.T) {
	owner := &User{ID: 1}
	admin := &User{ID: 2}
	userRepo := newFakeTeamUserRepo(owner, admin)
	memberRepo := &fakeTeamMemberRepo{}
	_, err := memberRepo.Create(context.Background(), owner.ID, admin.ID, domain.TeamMemberRoleAdmin)
	require.NoError(t, err)
	invRepo := &fakeTeamInvitationRepo{}
	inv, err := invRepo.Create(context.Background(), TeamInvitationCreateInput{OwnerUserID: owner.ID, InvitedEmail: "x@example.com", Token: "tok-x", ExpiresAt: time.Now().Add(time.Hour), Role: domain.TeamMemberRoleMember, QuotaMode: domain.TeamQuotaModeAllocated})
	require.NoError(t, err)
	svc := newTeamServiceForTest(userRepo, memberRepo, invRepo)

	err = svc.RevokeInvitation(context.Background(), owner.ID, admin.ID, inv.ID)
	require.NoError(t, err)
}

// --- RemoveMember ---

func TestTeamService_RemoveMember_RejectsNonMember(t *testing.T) {
	owner := &User{ID: 1}
	userRepo := newFakeTeamUserRepo(owner)
	svc := newTeamServiceForTest(userRepo, &fakeTeamMemberRepo{}, &fakeTeamInvitationRepo{})

	err := svc.RemoveMember(context.Background(), owner.ID, 999, 2)
	require.ErrorIs(t, err, ErrInsufficientPerms)
}

func TestTeamService_RemoveMember_RejectsRemovingOwner(t *testing.T) {
	owner := &User{ID: 1}
	userRepo := newFakeTeamUserRepo(owner)
	svc := newTeamServiceForTest(userRepo, &fakeTeamMemberRepo{}, &fakeTeamInvitationRepo{})

	err := svc.RemoveMember(context.Background(), owner.ID, owner.ID, owner.ID)
	require.ErrorIs(t, err, ErrTeamCannotRemoveOwner)
}

func TestTeamService_RemoveMember_ReturnsNotFoundForInactiveMember(t *testing.T) {
	owner := &User{ID: 1}
	userRepo := newFakeTeamUserRepo(owner)
	svc := newTeamServiceForTest(userRepo, &fakeTeamMemberRepo{}, &fakeTeamInvitationRepo{})

	err := svc.RemoveMember(context.Background(), owner.ID, owner.ID, 2)
	require.ErrorIs(t, err, ErrTeamMemberNotFound)
}

func TestTeamService_RemoveMember_AdminCannotRemoveAdmin(t *testing.T) {
	owner := &User{ID: 1}
	adminA := &User{ID: 2}
	adminB := &User{ID: 3}
	userRepo := newFakeTeamUserRepo(owner, adminA, adminB)
	memberRepo := &fakeTeamMemberRepo{}
	_, err := memberRepo.Create(context.Background(), owner.ID, adminA.ID, domain.TeamMemberRoleAdmin)
	require.NoError(t, err)
	_, err = memberRepo.Create(context.Background(), owner.ID, adminB.ID, domain.TeamMemberRoleAdmin)
	require.NoError(t, err)
	svc := newTeamServiceForTest(userRepo, memberRepo, &fakeTeamInvitationRepo{})

	err = svc.RemoveMember(context.Background(), owner.ID, adminA.ID, adminB.ID)
	require.ErrorIs(t, err, ErrTeamOwnerOnly)
}

func TestTeamService_RemoveMember_OwnerCanRemoveAdmin(t *testing.T) {
	owner := &User{ID: 1}
	admin := &User{ID: 2}
	userRepo := newFakeTeamUserRepo(owner, admin)
	memberRepo := &fakeTeamMemberRepo{}
	_, err := memberRepo.Create(context.Background(), owner.ID, admin.ID, domain.TeamMemberRoleAdmin)
	require.NoError(t, err)
	svc := newTeamServiceForTest(userRepo, memberRepo, &fakeTeamInvitationRepo{})

	err = svc.RemoveMember(context.Background(), owner.ID, owner.ID, admin.ID)
	require.NoError(t, err)
}

// --- ListMembers / ListMyTeams ---

func TestTeamService_ListMembers_RejectsNonMember(t *testing.T) {
	owner := &User{ID: 1, Email: "owner@example.com"}
	userRepo := newFakeTeamUserRepo(owner)
	svc := newTeamServiceForTest(userRepo, &fakeTeamMemberRepo{}, &fakeTeamInvitationRepo{})

	_, err := svc.ListMembers(context.Background(), owner.ID, 999)
	require.ErrorIs(t, err, ErrInsufficientPerms)
}

func TestTeamService_ListMembers_IncludesImplicitOwnerAndActiveMembers(t *testing.T) {
	owner := &User{ID: 1, Email: "owner@example.com"}
	member := &User{ID: 2, Email: "member@example.com"}
	userRepo := newFakeTeamUserRepo(owner, member)
	memberRepo := &fakeTeamMemberRepo{}
	_, err := memberRepo.Create(context.Background(), owner.ID, member.ID, domain.TeamMemberRoleMember)
	require.NoError(t, err)
	svc := newTeamServiceForTest(userRepo, memberRepo, &fakeTeamInvitationRepo{})

	views, err := svc.ListMembers(context.Background(), owner.ID, member.ID)
	require.NoError(t, err)
	require.Len(t, views, 2)
	require.Equal(t, "owner", views[0].Role)
	require.Equal(t, owner.Email, views[0].Email)
	require.Equal(t, domain.TeamMemberRoleMember, views[1].Role)
	require.Equal(t, member.Email, views[1].Email)
}

func TestTeamService_ListMyTeams_IncludesPersonalAndMemberships(t *testing.T) {
	owner := &User{ID: 1, Email: "owner@example.com"}
	member := &User{ID: 2, Email: "member@example.com", Balance: 50}
	userRepo := newFakeTeamUserRepo(owner, member)
	memberRepo := &fakeTeamMemberRepo{}
	_, err := memberRepo.Create(context.Background(), owner.ID, member.ID, domain.TeamMemberRoleMember)
	require.NoError(t, err)
	svc := newTeamServiceForTest(userRepo, memberRepo, &fakeTeamInvitationRepo{})

	teams, err := svc.ListMyTeams(context.Background(), member.ID)
	require.NoError(t, err)
	require.Len(t, teams, 2)
	require.True(t, teams[0].IsPersonal)
	require.Equal(t, member.ID, teams[0].OwnerUserID)
	require.Equal(t, member.Balance, teams[0].Balance)
	require.False(t, teams[1].IsPersonal)
	require.Equal(t, owner.ID, teams[1].OwnerUserID)
	require.Equal(t, domain.TeamMemberRoleMember, teams[1].Role)
	// member 无权查看企业余额：加入企业那一行不暴露 owner 的余额。
	require.Zero(t, teams[1].Balance)
}

// --- SetMemberRole / SetMemberDepartment / SetMemberQuotaSettings ---

func TestTeamService_SetMemberRole_OnlyOwner(t *testing.T) {
	owner := &User{ID: 1}
	admin := &User{ID: 2}
	member := &User{ID: 3}
	userRepo := newFakeTeamUserRepo(owner, admin, member)
	memberRepo := &fakeTeamMemberRepo{}
	_, err := memberRepo.Create(context.Background(), owner.ID, admin.ID, domain.TeamMemberRoleAdmin)
	require.NoError(t, err)
	_, err = memberRepo.Create(context.Background(), owner.ID, member.ID, domain.TeamMemberRoleMember)
	require.NoError(t, err)
	svc := newTeamServiceForTest(userRepo, memberRepo, &fakeTeamInvitationRepo{})

	err = svc.SetMemberRole(context.Background(), owner.ID, admin.ID, member.ID, domain.TeamMemberRoleAdmin)
	require.ErrorIs(t, err, ErrTeamOwnerOnly)

	err = svc.SetMemberRole(context.Background(), owner.ID, owner.ID, member.ID, domain.TeamMemberRoleAdmin)
	require.NoError(t, err)
}

func TestTeamService_SetMemberQuotaSettings_SharedRequiresValidLevels(t *testing.T) {
	owner := &User{ID: 1}
	member := &User{ID: 2}
	userRepo := newFakeTeamUserRepo(owner, member)
	memberRepo := &fakeTeamMemberRepo{}
	_, err := memberRepo.Create(context.Background(), owner.ID, member.ID, domain.TeamMemberRoleMember)
	require.NoError(t, err)
	svc := newTeamServiceForTest(userRepo, memberRepo, &fakeTeamInvitationRepo{})

	err = svc.SetMemberQuotaSettings(context.Background(), owner.ID, owner.ID, member.ID, domain.TeamQuotaModeShared, nil, nil)
	require.Error(t, err)

	threshold, target := 10.0, 50.0
	err = svc.SetMemberQuotaSettings(context.Background(), owner.ID, owner.ID, member.ID, domain.TeamQuotaModeShared, &threshold, &target)
	require.NoError(t, err)
	require.Equal(t, domain.TeamQuotaModeShared, memberRepo.members[0].QuotaMode)
}

func TestTeamService_SetMemberDepartment_RejectsUnknownDepartment(t *testing.T) {
	owner := &User{ID: 1}
	member := &User{ID: 2}
	userRepo := newFakeTeamUserRepo(owner, member)
	memberRepo := &fakeTeamMemberRepo{}
	_, err := memberRepo.Create(context.Background(), owner.ID, member.ID, domain.TeamMemberRoleMember)
	require.NoError(t, err)
	svc := NewTeamService(memberRepo, &fakeTeamInvitationRepo{}, nil, userRepo, nil, &fakeTeamDepartmentRepo{}, nil, nil, nil)

	missing := int64(1)
	err = svc.SetMemberDepartment(context.Background(), owner.ID, owner.ID, member.ID, &missing)
	require.ErrorIs(t, err, ErrTeamDepartmentNotFound)
}

// --- VerifyMemberAccess ---

func TestTeamService_VerifyMemberAccess_OwnerCanViewSelf(t *testing.T) {
	owner := &User{ID: 1}
	userRepo := newFakeTeamUserRepo(owner)
	svc := newTeamServiceForTest(userRepo, &fakeTeamMemberRepo{}, &fakeTeamInvitationRepo{})

	err := svc.VerifyMemberAccess(context.Background(), owner.ID, owner.ID, owner.ID)
	require.NoError(t, err)
}

func TestTeamService_VerifyMemberAccess_AdminCanViewActiveMember(t *testing.T) {
	owner := &User{ID: 1}
	admin := &User{ID: 2}
	member := &User{ID: 3}
	userRepo := newFakeTeamUserRepo(owner, admin, member)
	memberRepo := &fakeTeamMemberRepo{}
	_, err := memberRepo.Create(context.Background(), owner.ID, admin.ID, domain.TeamMemberRoleAdmin)
	require.NoError(t, err)
	_, err = memberRepo.Create(context.Background(), owner.ID, member.ID, domain.TeamMemberRoleMember)
	require.NoError(t, err)
	svc := newTeamServiceForTest(userRepo, memberRepo, &fakeTeamInvitationRepo{})

	err = svc.VerifyMemberAccess(context.Background(), owner.ID, admin.ID, member.ID)
	require.NoError(t, err)
}

func TestTeamService_VerifyMemberAccess_RejectsNonMemberActor(t *testing.T) {
	owner := &User{ID: 1}
	member := &User{ID: 2}
	userRepo := newFakeTeamUserRepo(owner, member)
	memberRepo := &fakeTeamMemberRepo{}
	_, err := memberRepo.Create(context.Background(), owner.ID, member.ID, domain.TeamMemberRoleMember)
	require.NoError(t, err)
	svc := newTeamServiceForTest(userRepo, memberRepo, &fakeTeamInvitationRepo{})

	// 999 既不是 owner 也不是该企业的 active 成员，不能查看任何人的统计（含企业 owner 本人）。
	err = svc.VerifyMemberAccess(context.Background(), owner.ID, 999, owner.ID)
	require.ErrorIs(t, err, ErrInsufficientPerms)
}

func TestTeamService_VerifyMemberAccess_RejectsCrossEnterpriseTarget(t *testing.T) {
	owner := &User{ID: 1}
	strangerOwner := &User{ID: 99}
	stranger := &User{ID: 100}
	userRepo := newFakeTeamUserRepo(owner, strangerOwner, stranger)
	memberRepo := &fakeTeamMemberRepo{}
	// stranger 是"另一家企业"的成员，不属于 owner=1 这家。
	_, err := memberRepo.Create(context.Background(), strangerOwner.ID, stranger.ID, domain.TeamMemberRoleMember)
	require.NoError(t, err)
	svc := newTeamServiceForTest(userRepo, memberRepo, &fakeTeamInvitationRepo{})

	// owner（企业1的所有者）尝试查看不属于自己企业的 stranger 的统计，必须被拒绝。
	err = svc.VerifyMemberAccess(context.Background(), owner.ID, owner.ID, stranger.ID)
	require.ErrorIs(t, err, ErrTeamMemberNotFound)
}
