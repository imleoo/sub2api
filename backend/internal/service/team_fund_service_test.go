//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/domain"
	"github.com/stretchr/testify/require"
)

func newFundTestSetup(ownerBalance, memberBalance float64) (*fakeTeamMemberRepo, *fakeTeamFundRepo, *User, *User) {
	owner := &User{ID: 1, Email: "owner@example.com", Balance: ownerBalance}
	member := &User{ID: 2, Email: "member@example.com", Balance: memberBalance}
	memberRepo := &fakeTeamMemberRepo{}
	_, _ = memberRepo.Create(context.Background(), owner.ID, member.ID, domain.TeamMemberRoleMember)
	fundRepo := &fakeTeamFundRepo{
		balances: map[int64]*float64{owner.ID: &owner.Balance, member.ID: &member.Balance},
		members:  memberRepo,
	}
	return memberRepo, fundRepo, owner, member
}

func TestTeamFundService_GrantToMember_RejectsNonMember(t *testing.T) {
	memberRepo, fundRepo, owner, member := newFundTestSetup(100, 0)
	userRepo := newFakeTeamUserRepo(owner, member)
	svc := NewTeamFundService(fundRepo, memberRepo, userRepo, nil, nil)

	_, err := svc.GrantToMember(context.Background(), owner.ID, 999, member.ID, 10, "")
	require.ErrorIs(t, err, ErrInsufficientPerms)
}

func TestTeamFundService_GrantToMember_Success(t *testing.T) {
	memberRepo, fundRepo, owner, member := newFundTestSetup(100, 0)
	userRepo := newFakeTeamUserRepo(owner, member)
	svc := NewTeamFundService(fundRepo, memberRepo, userRepo, nil, nil)

	t_, err := svc.GrantToMember(context.Background(), owner.ID, owner.ID, member.ID, 30, "initial")
	require.NoError(t, err)
	require.Equal(t, domain.TeamFundDirectionGrant, t_.Direction)
	require.Equal(t, 30.0, t_.Amount)
	require.Equal(t, 70.0, owner.Balance)
	require.Equal(t, 30.0, member.Balance)
	require.Equal(t, 30.0, memberRepo.members[0].GrantedNetUSD)
}

func TestTeamFundService_GrantToMember_InsufficientBalance(t *testing.T) {
	memberRepo, fundRepo, owner, member := newFundTestSetup(10, 0)
	userRepo := newFakeTeamUserRepo(owner, member)
	svc := NewTeamFundService(fundRepo, memberRepo, userRepo, nil, nil)

	_, err := svc.GrantToMember(context.Background(), owner.ID, owner.ID, member.ID, 30, "")
	require.ErrorIs(t, err, ErrTeamInsufficientBalance)
	require.Equal(t, 10.0, owner.Balance)
	require.Equal(t, 0.0, member.Balance)
}

func TestTeamFundService_GrantToMember_RejectsNonPositiveAmount(t *testing.T) {
	memberRepo, fundRepo, owner, member := newFundTestSetup(100, 0)
	userRepo := newFakeTeamUserRepo(owner, member)
	svc := NewTeamFundService(fundRepo, memberRepo, userRepo, nil, nil)

	_, err := svc.GrantToMember(context.Background(), owner.ID, owner.ID, member.ID, 0, "")
	require.ErrorIs(t, err, ErrTeamInvalidTransferAmount)

	_, err = svc.GrantToMember(context.Background(), owner.ID, owner.ID, member.ID, -5, "")
	require.ErrorIs(t, err, ErrTeamInvalidTransferAmount)
}

func TestTeamFundService_ReclaimFromMember_LimitedByGrantedNet(t *testing.T) {
	memberRepo, fundRepo, owner, member := newFundTestSetup(100, 0)
	userRepo := newFakeTeamUserRepo(owner, member)
	svc := NewTeamFundService(fundRepo, memberRepo, userRepo, nil, nil)

	_, err := svc.GrantToMember(context.Background(), owner.ID, owner.ID, member.ID, 20, "")
	require.NoError(t, err)

	// 员工自己又充值了 50，余额变成 70，但 granted_net 只有 20，回收上限仍是 20。
	member.Balance += 50
	*fundRepo.balances[member.ID] = member.Balance

	_, err = svc.ReclaimFromMember(context.Background(), owner.ID, owner.ID, member.ID, 21, "")
	require.ErrorIs(t, err, ErrTeamReclaimExceedsLimit)

	t_, err := svc.ReclaimFromMember(context.Background(), owner.ID, owner.ID, member.ID, 20, "")
	require.NoError(t, err)
	require.Equal(t, domain.TeamFundDirectionReclaim, t_.Direction)
	require.Equal(t, 0.0, memberRepo.members[0].GrantedNetUSD)
	require.Equal(t, 100.0, owner.Balance) // 100 - 20(grant) + 20(reclaim)
}

func TestTeamFundService_ReclaimFromMember_LimitedByMemberBalance(t *testing.T) {
	memberRepo, fundRepo, owner, member := newFundTestSetup(100, 0)
	userRepo := newFakeTeamUserRepo(owner, member)
	svc := NewTeamFundService(fundRepo, memberRepo, userRepo, nil, nil)

	_, err := svc.GrantToMember(context.Background(), owner.ID, owner.ID, member.ID, 50, "")
	require.NoError(t, err)

	// 员工已经花掉了一部分，只剩 10，即使 granted_net=50，回收上限也只能是 10。
	member.Balance = 10
	*fundRepo.balances[member.ID] = 10

	_, err = svc.ReclaimFromMember(context.Background(), owner.ID, owner.ID, member.ID, 11, "")
	require.ErrorIs(t, err, ErrTeamReclaimExceedsLimit)

	_, err = svc.ReclaimFromMember(context.Background(), owner.ID, owner.ID, member.ID, 10, "")
	require.NoError(t, err)
	require.Equal(t, 0.0, member.Balance)
}

func TestTeamFundService_ListTransfers_Pagination(t *testing.T) {
	memberRepo, fundRepo, owner, member := newFundTestSetup(1000, 0)
	userRepo := newFakeTeamUserRepo(owner, member)
	svc := NewTeamFundService(fundRepo, memberRepo, userRepo, nil, nil)

	for i := 0; i < 5; i++ {
		_, err := svc.GrantToMember(context.Background(), owner.ID, owner.ID, member.ID, 1, "")
		require.NoError(t, err)
	}

	page1, total, err := svc.ListTransfers(context.Background(), owner.ID, owner.ID, 0, 2)
	require.NoError(t, err)
	require.Equal(t, 5, total)
	require.Len(t, page1, 2)
	// 台账行须带成员/操作人 email（"划给了谁"），账号缺失时留空不阻断。
	require.Equal(t, member.Email, page1[0].MemberEmail)
	require.Equal(t, owner.Email, page1[0].OperatorEmail)
}

// 连续多笔 grant/reclaim 交替执行后，余额与 granted_net 的记账应始终保持不变量
// （二者总和守恒、granted_net 不为负）。fake repo 不建模真实并发/行锁——锁序防
// 死锁的正确性已在 team_fund_repo.go 中通过固定 min/max(user_id) 顺序保证，并在
// P8 端到端冒烟里对真实 Postgres 验证；这里只覆盖记账逻辑本身的正确性。
func TestTeamFundService_SequentialGrantReclaim_KeepsInvariants(t *testing.T) {
	memberRepo, fundRepo, owner, member := newFundTestSetup(1000, 100)
	userRepo := newFakeTeamUserRepo(owner, member)
	svc := NewTeamFundService(fundRepo, memberRepo, userRepo, nil, nil)
	initialTotal := owner.Balance + member.Balance

	_, err := svc.GrantToMember(context.Background(), owner.ID, owner.ID, member.ID, 100, "")
	require.NoError(t, err)
	for i := 0; i < 20; i++ {
		_, err := svc.GrantToMember(context.Background(), owner.ID, owner.ID, member.ID, 1, "")
		require.NoError(t, err)
		_, err = svc.ReclaimFromMember(context.Background(), owner.ID, owner.ID, member.ID, 1, "")
		require.NoError(t, err)
	}

	require.Equal(t, initialTotal, owner.Balance+member.Balance)
	require.GreaterOrEqual(t, owner.Balance, 0.0)
	require.GreaterOrEqual(t, member.Balance, 0.0)
	require.GreaterOrEqual(t, memberRepo.members[0].GrantedNetUSD, 0.0)
}

func TestTeamDepartmentService_Create_RejectsDuplicateName(t *testing.T) {
	owner := &User{ID: 1}
	userRepo := newFakeTeamUserRepo(owner)
	deptRepo := &fakeTeamDepartmentRepo{}
	svc := NewTeamDepartmentService(deptRepo, &fakeTeamMemberRepo{}, nil)
	_ = userRepo

	_, err := svc.Create(context.Background(), owner.ID, owner.ID, "Engineering", 0)
	require.NoError(t, err)

	_, err = svc.Create(context.Background(), owner.ID, owner.ID, "Engineering", 0)
	require.ErrorIs(t, err, ErrTeamDepartmentNameTaken)
}

func TestTeamDepartmentService_Delete_NotFound(t *testing.T) {
	owner := &User{ID: 1}
	deptRepo := &fakeTeamDepartmentRepo{}
	svc := NewTeamDepartmentService(deptRepo, &fakeTeamMemberRepo{}, nil)

	err := svc.Delete(context.Background(), owner.ID, owner.ID, 999)
	require.ErrorIs(t, err, ErrTeamDepartmentNotFound)
}

func TestEnterpriseService_Upgrade_RejectsDuplicate(t *testing.T) {
	repo := newFakeEnterpriseProfileRepo()
	svc := NewEnterpriseService(repo, nil, nil)

	_, err := svc.Upgrade(context.Background(), 1, EnterpriseUpgradeInput{CompanyName: "Acme"})
	require.NoError(t, err)

	_, err = svc.Upgrade(context.Background(), 1, EnterpriseUpgradeInput{CompanyName: "Acme Again"})
	require.ErrorIs(t, err, ErrEnterpriseAlreadyUpgraded)
}

func TestEnterpriseService_Upgrade_RejectsEmptyCompanyName(t *testing.T) {
	repo := newFakeEnterpriseProfileRepo()
	svc := NewEnterpriseService(repo, nil, nil)

	_, err := svc.Upgrade(context.Background(), 1, EnterpriseUpgradeInput{CompanyName: "  "})
	require.Error(t, err)
}

func TestEnterpriseService_GetProfile_NilForNonEnterprise(t *testing.T) {
	repo := newFakeEnterpriseProfileRepo()
	svc := NewEnterpriseService(repo, nil, nil)

	p, err := svc.GetProfile(context.Background(), 1)
	require.NoError(t, err)
	require.Nil(t, p)
}

func TestEnterpriseService_Upgrade_RejectsExistingTeamMember(t *testing.T) {
	memberRepo, _, owner, member := newFundTestSetup(1000, 0)
	_ = owner
	repo := newFakeEnterpriseProfileRepo()
	svc := NewEnterpriseService(repo, nil, memberRepo)

	// member 已是 owner 企业的员工：不允许再自助升级为企业客户。
	_, err := svc.Upgrade(context.Background(), member.ID, EnterpriseUpgradeInput{CompanyName: "X"})
	require.ErrorIs(t, err, ErrEnterpriseMemberCannotUpgrade)
}
