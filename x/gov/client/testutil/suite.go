package testutil

import (
	"encoding/base64"
	"fmt"
	"strings"

	"github.com/cosmos/cosmos-sdk/testutil"

	"github.com/gogo/protobuf/proto"
	"github.com/stretchr/testify/suite"

	tmcli "github.com/tendermint/tendermint/libs/cli"

	"github.com/cosmos/cosmos-sdk/client/flags"
	clitestutil "github.com/cosmos/cosmos-sdk/testutil/cli"
	"github.com/cosmos/cosmos-sdk/testutil/network"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	"github.com/cosmos/cosmos-sdk/x/gov/client/cli"
	"github.com/cosmos/cosmos-sdk/x/gov/types"
	"github.com/cosmos/cosmos-sdk/x/gov/types/v1beta1"
	"github.com/cosmos/cosmos-sdk/x/gov/types/v1beta2"
)

type IntegrationTestSuite struct {
	suite.Suite

	cfg     network.Config
	network *network.Network
}

func NewIntegrationTestSuite(cfg network.Config) *IntegrationTestSuite {
	return &IntegrationTestSuite{cfg: cfg}
}

func (s *IntegrationTestSuite) SetupSuite() {
	s.T().Log("setting up integration test suite")

	var err error
	s.network, err = network.New(s.T(), s.T().TempDir(), s.cfg)
	s.Require().NoError(err)

	_, err = s.network.WaitForHeight(1)
	s.Require().NoError(err)

	val := s.network.Validators[0]

	// create a proposal with deposit
	_, err = MsgSubmitLegacyProposal(val.ClientCtx, val.Address.String(),
		"Text Proposal 1", "Where is the title!?", v1beta1.ProposalTypeText,
		fmt.Sprintf("--%s=%s", cli.FlagDeposit, sdk.NewCoin(s.cfg.BondDenom, v1beta2.DefaultMinDepositTokens).String()))
	s.Require().NoError(err)
	_, err = s.network.WaitForHeight(1)
	s.Require().NoError(err)

	// vote for proposal
	_, err = MsgVote(val.ClientCtx, val.Address.String(), "1", "yes")
	s.Require().NoError(err)

	// create a proposal without deposit
	_, err = MsgSubmitLegacyProposal(val.ClientCtx, val.Address.String(),
		"Text Proposal 2", "Where is the title!?", v1beta1.ProposalTypeText)
	s.Require().NoError(err)
	_, err = s.network.WaitForHeight(1)
	s.Require().NoError(err)

	// create a proposal3 with deposit
	_, err = MsgSubmitLegacyProposal(val.ClientCtx, val.Address.String(),
		"Text Proposal 3", "Where is the title!?", v1beta1.ProposalTypeText,
		fmt.Sprintf("--%s=%s", cli.FlagDeposit, sdk.NewCoin(s.cfg.BondDenom, v1beta2.DefaultMinDepositTokens).String()))
	s.Require().NoError(err)
	_, err = s.network.WaitForHeight(1)
	s.Require().NoError(err)

	// vote for proposal3 as val
	_, err = MsgVote(val.ClientCtx, val.Address.String(), "3", "yes=0.6,no=0.3,abstain=0.05,no_with_veto=0.05")
	s.Require().NoError(err)
}

func (s *IntegrationTestSuite) TearDownSuite() {
	s.T().Log("tearing down integration test suite")
	s.network.Cleanup()
}

func (s *IntegrationTestSuite) TestCmdParams() {
	val := s.network.Validators[0]

	testCases := []struct {
		name           string
		args           []string
		expectedOutput string
	}{
		{
			"json output",
			[]string{fmt.Sprintf("--%s=json", tmcli.OutputFlag)},
			`{"voting_params":{"voting_period":"172800000000000"},"tally_params":{"quorum":"0.334000000000000000","threshold":"0.500000000000000000","veto_threshold":"0.334000000000000000"},"deposit_params":{"min_deposit":[{"denom":"stake","amount":"10000000"}],"max_deposit_period":"172800000000000"}}`,
		},
		{
			"text output",
			[]string{},
			`
deposit_params:
  max_deposit_period: "172800000000000"
  min_deposit:
  - amount: "10000000"
    denom: stake
tally_params:
  quorum: "0.334000000000000000"
  threshold: "0.500000000000000000"
  veto_threshold: "0.334000000000000000"
voting_params:
  voting_period: "172800000000000"
	`,
		},
	}

	for _, tc := range testCases {
		tc := tc

		s.Run(tc.name, func() {
			cmd := cli.GetCmdQueryParams()
			clientCtx := val.ClientCtx

			out, err := clitestutil.ExecTestCLICmd(clientCtx, cmd, tc.args)
			s.Require().NoError(err)
			s.Require().Equal(strings.TrimSpace(tc.expectedOutput), strings.TrimSpace(out.String()))
		})
	}
}

func (s *IntegrationTestSuite) TestCmdParam() {
	val := s.network.Validators[0]

	testCases := []struct {
		name           string
		args           []string
		expectedOutput string
	}{
		{
			"voting params",
			[]string{
				"voting",
				fmt.Sprintf("--%s=json", tmcli.OutputFlag),
			},
			`{"voting_period":"172800000000000"}`,
		},
		{
			"tally params",
			[]string{
				"tallying",
				fmt.Sprintf("--%s=json", tmcli.OutputFlag),
			},
			`{"quorum":"0.334000000000000000","threshold":"0.500000000000000000","veto_threshold":"0.334000000000000000"}`,
		},
		{
			"deposit params",
			[]string{
				"deposit",
				fmt.Sprintf("--%s=json", tmcli.OutputFlag),
			},
			`{"min_deposit":[{"denom":"stake","amount":"10000000"}],"max_deposit_period":"172800000000000"}`,
		},
	}

	for _, tc := range testCases {
		tc := tc

		s.Run(tc.name, func() {
			cmd := cli.GetCmdQueryParam()
			clientCtx := val.ClientCtx

			out, err := clitestutil.ExecTestCLICmd(clientCtx, cmd, tc.args)
			s.Require().NoError(err)
			s.Require().Equal(strings.TrimSpace(tc.expectedOutput), strings.TrimSpace(out.String()))
		})
	}
}

func (s *IntegrationTestSuite) TestCmdProposer() {
	val := s.network.Validators[0]

	testCases := []struct {
		name           string
		args           []string
		expectErr      bool
		expectedOutput string
	}{
		{
			"without proposal id",
			[]string{
				fmt.Sprintf("--%s=json", tmcli.OutputFlag),
			},
			true,
			``,
		},
		{
			"json output",
			[]string{
				"1",
				fmt.Sprintf("--%s=json", tmcli.OutputFlag),
			},
			false,
			fmt.Sprintf("{\"proposal_id\":\"%s\",\"proposer\":\"%s\"}", "1", val.Address.String()),
		},
	}

	for _, tc := range testCases {
		tc := tc

		s.Run(tc.name, func() {
			cmd := cli.GetCmdQueryProposer()
			clientCtx := val.ClientCtx
			out, err := clitestutil.ExecTestCLICmd(clientCtx, cmd, tc.args)

			if tc.expectErr {
				s.Require().Error(err)
			} else {
				s.Require().NoError(err)
				s.Require().Equal(strings.TrimSpace(tc.expectedOutput), strings.TrimSpace(out.String()))
			}
		})
	}
}

func (s *IntegrationTestSuite) TestCmdTally() {
	val := s.network.Validators[0]

	testCases := []struct {
		name           string
		args           []string
		expectErr      bool
		expectedOutput v1beta2.TallyResult
	}{
		{
			"without proposal id",
			[]string{
				fmt.Sprintf("--%s=json", tmcli.OutputFlag),
			},
			true,
			v1beta2.TallyResult{},
		},
		{
			"json output",
			[]string{
				"2",
				fmt.Sprintf("--%s=json", tmcli.OutputFlag),
			},
			false,
			v1beta2.NewTallyResult(sdk.NewInt(0), sdk.NewInt(0), sdk.NewInt(0), sdk.NewInt(0)),
		},
		{
			"json output",
			[]string{
				"1",
				fmt.Sprintf("--%s=json", tmcli.OutputFlag),
			},
			false,
			v1beta2.NewTallyResult(s.cfg.BondedTokens, sdk.NewInt(0), sdk.NewInt(0), sdk.NewInt(0)),
		},
	}

	for _, tc := range testCases {
		tc := tc

		s.Run(tc.name, func() {
			cmd := cli.GetCmdQueryTally()
			clientCtx := val.ClientCtx
			out, err := clitestutil.ExecTestCLICmd(clientCtx, cmd, tc.args)

			if tc.expectErr {
				s.Require().Error(err)
			} else {
				var tally v1beta2.TallyResult
				s.Require().NoError(clientCtx.Codec.UnmarshalJSON(out.Bytes(), &tally), out.String())
				s.Require().Equal(tally, tc.expectedOutput)
			}
		})
	}
}

func (s *IntegrationTestSuite) TestNewCmdSubmitProposal() {
	val := s.network.Validators[0]

	// Create an legacy proposal JSON, make sure it doesn't pass this new CLI
	// command.
	invalidProp := `{
	"title": "",
	"description": "Where is the title!?",
	"type": "Text",
	"deposit": "-324foocoin"
}`
	invalidPropFile := testutil.WriteToNewTempFile(s.T(), invalidProp)

	// Create a valid new proposal JSON.
	propMetadata := []byte{42}
	validProp := fmt.Sprintf(`
{
	"messages": [
		{
			"@type": "/cosmos.gov.v1beta2.MsgExecLegacyContent",
			"authority": "%s",
			"content": {
				"@type": "/cosmos.gov.v1beta1.TextProposal",
				"title": "My awesome title",
				"description": "My awesome description"
			}
		}
	],
	"metadata": "%s",
	"deposit": "%s"
}`, authtypes.NewModuleAddress(types.ModuleName), base64.StdEncoding.EncodeToString(propMetadata), sdk.NewCoin(s.cfg.BondDenom, sdk.NewInt(5431)))
	validPropFile := testutil.WriteToNewTempFile(s.T(), validProp)

	testCases := []struct {
		name         string
		args         []string
		expectErr    bool
		expectedCode uint32
		respType     proto.Message
	}{
		{
			"invalid proposal",
			[]string{
				invalidPropFile.Name(),
				fmt.Sprintf("--%s=true", flags.FlagSkipConfirmation),
				fmt.Sprintf("--%s=%s", flags.FlagFees, sdk.NewCoins(sdk.NewCoin(s.cfg.BondDenom, sdk.NewInt(10))).String()),
			},
			true, 0, nil,
		},
		{
			"valid proposal",
			[]string{
				validPropFile.Name(),
				fmt.Sprintf("--%s=%s", flags.FlagFrom, val.Address.String()),
				fmt.Sprintf("--%s=true", flags.FlagSkipConfirmation),
				fmt.Sprintf("--%s=%s", flags.FlagBroadcastMode, flags.BroadcastBlock),
				fmt.Sprintf("--%s=%s", flags.FlagFees, sdk.NewCoins(sdk.NewCoin(s.cfg.BondDenom, sdk.NewInt(10))).String()),
			},
			false, 0, &sdk.TxResponse{},
		},
	}

	for _, tc := range testCases {
		tc := tc

		s.Run(tc.name, func() {
			cmd := cli.NewCmdSubmitProposal()
			clientCtx := val.ClientCtx

			out, err := clitestutil.ExecTestCLICmd(clientCtx, cmd, tc.args)
			if tc.expectErr {
				s.Require().Error(err)
			} else {
				s.Require().NoError(err)
				s.Require().NoError(clientCtx.Codec.UnmarshalJSON(out.Bytes(), tc.respType), out.String())
				txResp := tc.respType.(*sdk.TxResponse)
				s.Require().Equal(tc.expectedCode, txResp.Code, out.String())
			}
		})
	}
}

func (s *IntegrationTestSuite) TestNewCmdSubmitLegacyProposal() {
	val := s.network.Validators[0]
	invalidProp := `{
  "title": "",
	"description": "Where is the title!?",
	"type": "Text",
  "deposit": "-324foocoin"
}`
	invalidPropFile := testutil.WriteToNewTempFile(s.T(), invalidProp)
	validProp := fmt.Sprintf(`{
  "title": "Text Proposal",
	"description": "Hello, World!",
	"type": "Text",
  "deposit": "%s"
}`, sdk.NewCoin(s.cfg.BondDenom, sdk.NewInt(5431)))
	validPropFile := testutil.WriteToNewTempFile(s.T(), validProp)
	testCases := []struct {
		name         string
		args         []string
		expectErr    bool
		expectedCode uint32
		respType     proto.Message
	}{
		{
			"invalid proposal (file)",
			[]string{
				fmt.Sprintf("--%s=%s", cli.FlagProposal, invalidPropFile.Name()),
				fmt.Sprintf("--%s=%s", flags.FlagFrom, val.Address.String()),
				fmt.Sprintf("--%s=true", flags.FlagSkipConfirmation),
				fmt.Sprintf("--%s=%s", flags.FlagFees, sdk.NewCoins(sdk.NewCoin(s.cfg.BondDenom, sdk.NewInt(10))).String()),
			},
			true, 0, nil,
		},
		{
			"invalid proposal",
			[]string{
				fmt.Sprintf("--%s='Where is the title!?'", cli.FlagDescription),
				fmt.Sprintf("--%s=%s", cli.FlagProposalType, v1beta1.ProposalTypeText),
				fmt.Sprintf("--%s=%s", cli.FlagDeposit, sdk.NewCoin(s.cfg.BondDenom, sdk.NewInt(5431)).String()),
				fmt.Sprintf("--%s=%s", flags.FlagFrom, val.Address.String()),
				fmt.Sprintf("--%s=true", flags.FlagSkipConfirmation),
				fmt.Sprintf("--%s=%s", flags.FlagFees, sdk.NewCoins(sdk.NewCoin(s.cfg.BondDenom, sdk.NewInt(10))).String()),
			},
			true, 0, nil,
		},
		{
			"valid transaction (file)",
			[]string{
				fmt.Sprintf("--%s=%s", cli.FlagProposal, validPropFile.Name()),
				fmt.Sprintf("--%s=%s", flags.FlagFrom, val.Address.String()),
				fmt.Sprintf("--%s=true", flags.FlagSkipConfirmation),
				fmt.Sprintf("--%s=%s", flags.FlagBroadcastMode, flags.BroadcastBlock),
				fmt.Sprintf("--%s=%s", flags.FlagFees, sdk.NewCoins(sdk.NewCoin(s.cfg.BondDenom, sdk.NewInt(10))).String()),
			},
			false, 0, &sdk.TxResponse{},
		},
		{
			"valid transaction",
			[]string{
				fmt.Sprintf("--%s='Text Proposal'", cli.FlagTitle),
				fmt.Sprintf("--%s='Where is the title!?'", cli.FlagDescription),
				fmt.Sprintf("--%s=%s", cli.FlagProposalType, v1beta1.ProposalTypeText),
				fmt.Sprintf("--%s=%s", cli.FlagDeposit, sdk.NewCoin(s.cfg.BondDenom, sdk.NewInt(5431)).String()),
				fmt.Sprintf("--%s=%s", flags.FlagFrom, val.Address.String()),
				fmt.Sprintf("--%s=true", flags.FlagSkipConfirmation),
				fmt.Sprintf("--%s=%s", flags.FlagBroadcastMode, flags.BroadcastBlock),
				fmt.Sprintf("--%s=%s", flags.FlagFees, sdk.NewCoins(sdk.NewCoin(s.cfg.BondDenom, sdk.NewInt(10))).String()),
			},
			false, 0, &sdk.TxResponse{},
		},
	}

	for _, tc := range testCases {
		tc := tc

		s.Run(tc.name, func() {
			cmd := cli.NewCmdSubmitLegacyProposal()
			clientCtx := val.ClientCtx

			out, err := clitestutil.ExecTestCLICmd(clientCtx, cmd, tc.args)
			if tc.expectErr {
				s.Require().Error(err)
			} else {
				s.Require().NoError(err)
				s.Require().NoError(clientCtx.Codec.UnmarshalJSON(out.Bytes(), tc.respType), out.String())
				txResp := tc.respType.(*sdk.TxResponse)
				s.Require().Equal(tc.expectedCode, txResp.Code, out.String())
			}
		})
	}
}

func (s *IntegrationTestSuite) TestCmdGetProposal() {
	val := s.network.Validators[0]

	title := "Text Proposal 1"

	testCases := []struct {
		name      string
		args      []string
		expectErr bool
	}{
		{
			"get non existing proposal",
			[]string{
				"10",
				fmt.Sprintf("--%s=json", tmcli.OutputFlag),
			},
			true,
		},
		{
			"get proposal with json response",
			[]string{
				"1",
				fmt.Sprintf("--%s=json", tmcli.OutputFlag),
			},
			false,
		},
	}

	for _, tc := range testCases {
		tc := tc

		s.Run(tc.name, func() {
			cmd := cli.GetCmdQueryProposal()
			clientCtx := val.ClientCtx

			out, err := clitestutil.ExecTestCLICmd(clientCtx, cmd, tc.args)
			if tc.expectErr {
				s.Require().Error(err)
			} else {
				s.Require().NoError(err)
				var proposal v1beta2.Proposal
				s.Require().NoError(clientCtx.Codec.UnmarshalJSON(out.Bytes(), &proposal), out.String())
				s.Require().Equal(title, proposal.Messages[0].GetCachedValue().(*v1beta2.MsgExecLegacyContent).Content.GetCachedValue().(v1beta1.Content).GetTitle())
			}
		})
	}
}

func (s *IntegrationTestSuite) TestCmdGetProposals() {
	val := s.network.Validators[0]

	testCases := []struct {
		name      string
		args      []string
		expectErr bool
	}{
		{
			"get proposals as json response",
			[]string{
				fmt.Sprintf("--%s=json", tmcli.OutputFlag),
			},
			false,
		},
		{
			"get proposals with invalid status",
			[]string{
				"--status=unknown",
				fmt.Sprintf("--%s=json", tmcli.OutputFlag),
			},
			true,
		},
	}

	for _, tc := range testCases {
		tc := tc

		s.Run(tc.name, func() {
			cmd := cli.GetCmdQueryProposals()
			clientCtx := val.ClientCtx

			out, err := clitestutil.ExecTestCLICmd(clientCtx, cmd, tc.args)
			if tc.expectErr {
				s.Require().Error(err)
			} else {
				s.Require().NoError(err)
				var proposals v1beta2.QueryProposalsResponse

				s.Require().NoError(clientCtx.Codec.UnmarshalJSON(out.Bytes(), &proposals), out.String())
				s.Require().Len(proposals.Proposals, 3)
			}
		})
	}
}

func (s *IntegrationTestSuite) TestCmdQueryDeposits() {
	val := s.network.Validators[0]

	testCases := []struct {
		name      string
		args      []string
		expectErr bool
	}{
		{
			"get deposits of non existing proposal",
			[]string{
				"10",
			},
			true,
		},
		{
			"get deposits(valid req)",
			[]string{
				"1",
				fmt.Sprintf("--%s=json", tmcli.OutputFlag),
			},
			false,
		},
	}

	for _, tc := range testCases {
		tc := tc
		s.Run(tc.name, func() {
			cmd := cli.GetCmdQueryDeposits()
			clientCtx := val.ClientCtx

			out, err := clitestutil.ExecTestCLICmd(clientCtx, cmd, tc.args)

			if tc.expectErr {
				s.Require().Error(err)
			} else {
				s.Require().NoError(err)

				var deposits v1beta2.QueryDepositsResponse
				s.Require().NoError(clientCtx.Codec.UnmarshalJSON(out.Bytes(), &deposits), out.String())
				s.Require().Len(deposits.Deposits, 1)
			}
		})
	}
}

func (s *IntegrationTestSuite) TestCmdQueryDeposit() {
	val := s.network.Validators[0]
	depositAmount := sdk.NewCoin(s.cfg.BondDenom, v1beta2.DefaultMinDepositTokens)

	testCases := []struct {
		name      string
		args      []string
		expectErr bool
	}{
		{
			"get deposit with no depositer",
			[]string{
				"1",
			},
			true,
		},
		{
			"get deposit with wrong deposit address",
			[]string{
				"1",
				"wrong address",
			},
			true,
		},
		{
			"get deposit (valid req)",
			[]string{
				"1",
				val.Address.String(),
				fmt.Sprintf("--%s=json", tmcli.OutputFlag),
			},
			false,
		},
	}

	for _, tc := range testCases {
		tc := tc
		s.Run(tc.name, func() {
			cmd := cli.GetCmdQueryDeposit()
			clientCtx := val.ClientCtx

			out, err := clitestutil.ExecTestCLICmd(clientCtx, cmd, tc.args)

			if tc.expectErr {
				s.Require().Error(err)
			} else {
				s.Require().NoError(err)

				var deposit v1beta2.Deposit
				s.Require().NoError(clientCtx.Codec.UnmarshalJSON(out.Bytes(), &deposit), out.String())
				s.Require().Equal(depositAmount.String(), sdk.Coins(deposit.Amount).String())
			}
		})
	}
}

func (s *IntegrationTestSuite) TestNewCmdDeposit() {
	val := s.network.Validators[0]

	testCases := []struct {
		name         string
		args         []string
		expectErr    bool
		expectedCode uint32
	}{
		{
			"without proposal id",
			[]string{
				sdk.NewCoin(s.cfg.BondDenom, sdk.NewInt(10)).String(), // 10stake
				fmt.Sprintf("--%s=%s", flags.FlagFrom, val.Address.String()),
				fmt.Sprintf("--%s=true", flags.FlagSkipConfirmation),
				fmt.Sprintf("--%s=%s", flags.FlagBroadcastMode, flags.BroadcastBlock),
				fmt.Sprintf("--%s=%s", flags.FlagFees, sdk.NewCoins(sdk.NewCoin(s.cfg.BondDenom, sdk.NewInt(10))).String()),
			},
			true, 0,
		},
		{
			"without deposit amount",
			[]string{
				"1",
				fmt.Sprintf("--%s=%s", flags.FlagFrom, val.Address.String()),
				fmt.Sprintf("--%s=true", flags.FlagSkipConfirmation),
				fmt.Sprintf("--%s=%s", flags.FlagBroadcastMode, flags.BroadcastBlock),
				fmt.Sprintf("--%s=%s", flags.FlagFees, sdk.NewCoins(sdk.NewCoin(s.cfg.BondDenom, sdk.NewInt(10))).String()),
			},
			true, 0,
		},
		{
			"deposit on non existing proposal",
			[]string{
				"10",
				sdk.NewCoin(s.cfg.BondDenom, sdk.NewInt(10)).String(), // 10stake
				fmt.Sprintf("--%s=%s", flags.FlagFrom, val.Address.String()),
				fmt.Sprintf("--%s=true", flags.FlagSkipConfirmation),
				fmt.Sprintf("--%s=%s", flags.FlagBroadcastMode, flags.BroadcastBlock),
				fmt.Sprintf("--%s=%s", flags.FlagFees, sdk.NewCoins(sdk.NewCoin(s.cfg.BondDenom, sdk.NewInt(10))).String()),
			},
			false, 2,
		},
		{
			"deposit on non existing proposal",
			[]string{
				"1",
				sdk.NewCoin(s.cfg.BondDenom, sdk.NewInt(10)).String(), // 10stake
				fmt.Sprintf("--%s=%s", flags.FlagFrom, val.Address.String()),
				fmt.Sprintf("--%s=true", flags.FlagSkipConfirmation),
				fmt.Sprintf("--%s=%s", flags.FlagBroadcastMode, flags.BroadcastBlock),
				fmt.Sprintf("--%s=%s", flags.FlagFees, sdk.NewCoins(sdk.NewCoin(s.cfg.BondDenom, sdk.NewInt(10))).String()),
			},
			false, 0,
		},
	}

	for _, tc := range testCases {
		tc := tc
		var resp sdk.TxResponse

		s.Run(tc.name, func() {
			cmd := cli.NewCmdDeposit()
			clientCtx := val.ClientCtx

			out, err := clitestutil.ExecTestCLICmd(clientCtx, cmd, tc.args)
			if tc.expectErr {
				s.Require().Error(err)
			} else {
				s.Require().NoError(err)

				s.Require().NoError(clientCtx.Codec.UnmarshalJSON(out.Bytes(), &resp), out.String())
				s.Require().Equal(tc.expectedCode, resp.Code, out.String())
			}
		})
	}
}

func (s *IntegrationTestSuite) TestCmdQueryVotes() {
	val := s.network.Validators[0]

	testCases := []struct {
		name      string
		args      []string
		expectErr bool
	}{
		{
			"get votes with no proposal id",
			[]string{},
			true,
		},
		{
			"get votes of non existed proposal",
			[]string{
				"10",
			},
			true,
		},
		{
			"vote for invalid proposal",
			[]string{
				"1",
				fmt.Sprintf("--%s=json", tmcli.OutputFlag),
			},
			false,
		},
	}

	for _, tc := range testCases {
		tc := tc
		s.Run(tc.name, func() {
			cmd := cli.GetCmdQueryVotes()
			clientCtx := val.ClientCtx

			out, err := clitestutil.ExecTestCLICmd(clientCtx, cmd, tc.args)

			if tc.expectErr {
				s.Require().Error(err)
			} else {
				s.Require().NoError(err)

				var votes v1beta2.QueryVotesResponse
				s.Require().NoError(clientCtx.Codec.UnmarshalJSON(out.Bytes(), &votes), out.String())
				s.Require().Len(votes.Votes, 1)
			}
		})
	}
}

func (s *IntegrationTestSuite) TestCmdQueryVote() {
	val := s.network.Validators[0]

	testCases := []struct {
		name           string
		args           []string
		expectErr      bool
		expVoteOptions v1beta2.WeightedVoteOptions
	}{
		{
			"get vote of non existing proposal",
			[]string{
				"10",
				val.Address.String(),
			},
			true,
			v1beta2.NewNonSplitVoteOption(v1beta2.OptionYes),
		},
		{
			"get vote by wrong voter",
			[]string{
				"1",
				"wrong address",
			},
			true,
			v1beta2.NewNonSplitVoteOption(v1beta2.OptionYes),
		},
		{
			"vote for valid proposal",
			[]string{
				"1",
				val.Address.String(),
				fmt.Sprintf("--%s=json", tmcli.OutputFlag),
			},
			false,
			v1beta2.NewNonSplitVoteOption(v1beta2.OptionYes),
		},
		{
			"split vote for valid proposal",
			[]string{
				"3",
				val.Address.String(),
				fmt.Sprintf("--%s=json", tmcli.OutputFlag),
			},
			false,
			v1beta2.WeightedVoteOptions{
				&v1beta2.WeightedVoteOption{Option: v1beta2.OptionYes, Weight: sdk.NewDecWithPrec(60, 2).String()},
				&v1beta2.WeightedVoteOption{Option: v1beta2.OptionNo, Weight: sdk.NewDecWithPrec(30, 2).String()},
				&v1beta2.WeightedVoteOption{Option: v1beta2.OptionAbstain, Weight: sdk.NewDecWithPrec(5, 2).String()},
				&v1beta2.WeightedVoteOption{Option: v1beta2.OptionNoWithVeto, Weight: sdk.NewDecWithPrec(5, 2).String()},
			},
		},
	}

	for _, tc := range testCases {
		tc := tc
		s.Run(tc.name, func() {
			cmd := cli.GetCmdQueryVote()
			clientCtx := val.ClientCtx

			out, err := clitestutil.ExecTestCLICmd(clientCtx, cmd, tc.args)

			if tc.expectErr {
				s.Require().Error(err)
			} else {
				s.Require().NoError(err)

				var vote v1beta2.Vote
				s.Require().NoError(clientCtx.Codec.UnmarshalJSON(out.Bytes(), &vote), out.String())
				s.Require().Equal(len(vote.Options), len(tc.expVoteOptions))
				for i, option := range tc.expVoteOptions {
					s.Require().Equal(option.Option, vote.Options[i].Option)
					s.Require().Equal(option.Weight, vote.Options[i].Weight)
				}
			}
		})
	}
}

func (s *IntegrationTestSuite) TestNewCmdVote() {
	val := s.network.Validators[0]

	testCases := []struct {
		name         string
		args         []string
		expectErr    bool
		expectedCode uint32
	}{
		{
			"invalid vote",
			[]string{},
			true, 0,
		},
		{
			"vote for invalid proposal",
			[]string{
				"10",
				"yes",
				fmt.Sprintf("--%s=%s", flags.FlagFrom, val.Address.String()),
				fmt.Sprintf("--%s=true", flags.FlagSkipConfirmation),
				fmt.Sprintf("--%s=%s", flags.FlagBroadcastMode, flags.BroadcastBlock),
				fmt.Sprintf("--%s=%s", flags.FlagFees, sdk.NewCoins(sdk.NewCoin(s.cfg.BondDenom, sdk.NewInt(10))).String()),
			},
			false, 2,
		},
		{
			"valid vote",
			[]string{
				"1",
				"yes",
				fmt.Sprintf("--%s=%s", flags.FlagFrom, val.Address.String()),
				fmt.Sprintf("--%s=true", flags.FlagSkipConfirmation),
				fmt.Sprintf("--%s=%s", flags.FlagBroadcastMode, flags.BroadcastBlock),
				fmt.Sprintf("--%s=%s", flags.FlagFees, sdk.NewCoins(sdk.NewCoin(s.cfg.BondDenom, sdk.NewInt(10))).String()),
			},
			false, 0,
		},
	}

	for _, tc := range testCases {
		tc := tc
		s.Run(tc.name, func() {
			cmd := cli.NewCmdVote()
			clientCtx := val.ClientCtx
			var txResp sdk.TxResponse

			out, err := clitestutil.ExecTestCLICmd(clientCtx, cmd, tc.args)

			if tc.expectErr {
				s.Require().Error(err)
			} else {
				s.Require().NoError(err)
				s.Require().NoError(clientCtx.Codec.UnmarshalJSON(out.Bytes(), &txResp), out.String())
				s.Require().Equal(tc.expectedCode, txResp.Code, out.String())
			}
		})
	}
}

func (s *IntegrationTestSuite) TestNewCmdWeightedVote() {
	val := s.network.Validators[0]

	testCases := []struct {
		name         string
		args         []string
		expectErr    bool
		expectedCode uint32
	}{
		{
			"invalid vote",
			[]string{},
			true, 0,
		},
		{
			"vote for invalid proposal",
			[]string{
				"10",
				"yes",
				fmt.Sprintf("--%s=%s", flags.FlagFrom, val.Address.String()),
				fmt.Sprintf("--%s=true", flags.FlagSkipConfirmation),
				fmt.Sprintf("--%s=%s", flags.FlagBroadcastMode, flags.BroadcastBlock),
				fmt.Sprintf("--%s=%s", flags.FlagFees, sdk.NewCoins(sdk.NewCoin(s.cfg.BondDenom, sdk.NewInt(10))).String()),
			},
			false, 2,
		},
		{
			"valid vote",
			[]string{
				"1",
				"yes",
				fmt.Sprintf("--%s=%s", flags.FlagFrom, val.Address.String()),
				fmt.Sprintf("--%s=true", flags.FlagSkipConfirmation),
				fmt.Sprintf("--%s=%s", flags.FlagBroadcastMode, flags.BroadcastBlock),
				fmt.Sprintf("--%s=%s", flags.FlagFees, sdk.NewCoins(sdk.NewCoin(s.cfg.BondDenom, sdk.NewInt(10))).String()),
			},
			false, 0,
		},
		{
			"invalid valid split vote string",
			[]string{
				"1",
				"yes/0.6,no/0.3,abstain/0.05,no_with_veto/0.05",
				fmt.Sprintf("--%s=%s", flags.FlagFrom, val.Address.String()),
				fmt.Sprintf("--%s=true", flags.FlagSkipConfirmation),
				fmt.Sprintf("--%s=%s", flags.FlagBroadcastMode, flags.BroadcastBlock),
				fmt.Sprintf("--%s=%s", flags.FlagFees, sdk.NewCoins(sdk.NewCoin(s.cfg.BondDenom, sdk.NewInt(10))).String()),
			},
			true, 0,
		},
		{
			"valid split vote",
			[]string{
				"1",
				"yes=0.6,no=0.3,abstain=0.05,no_with_veto=0.05",
				fmt.Sprintf("--%s=%s", flags.FlagFrom, val.Address.String()),
				fmt.Sprintf("--%s=true", flags.FlagSkipConfirmation),
				fmt.Sprintf("--%s=%s", flags.FlagBroadcastMode, flags.BroadcastBlock),
				fmt.Sprintf("--%s=%s", flags.FlagFees, sdk.NewCoins(sdk.NewCoin(s.cfg.BondDenom, sdk.NewInt(10))).String()),
			},
			false, 0,
		},
	}

	for _, tc := range testCases {
		tc := tc
		s.Run(tc.name, func() {
			cmd := cli.NewCmdWeightedVote()
			clientCtx := val.ClientCtx
			var txResp sdk.TxResponse

			out, err := clitestutil.ExecTestCLICmd(clientCtx, cmd, tc.args)

			if tc.expectErr {
				s.Require().Error(err)
			} else {
				s.Require().NoError(err)
				s.Require().NoError(clientCtx.Codec.UnmarshalJSON(out.Bytes(), &txResp), out.String())
				s.Require().Equal(tc.expectedCode, txResp.Code, out.String())
			}
		})
	}
}
