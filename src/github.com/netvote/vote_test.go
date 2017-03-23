package main

import (
	"fmt"
	"testing"
	"github.com/hyperledger/fabric/core/chaincode/shim"
	"os"
)

const CREATE_DECISION_JSON = `{"Id":"test-id","Name":"What is your decision?","BallotId":"transaction-id","Options":[{"Id":"a","Name":"A","Description":"","Attributes":{"image":"/url"}}],"Attributes":{"Key":"Value"}}`
const CREATE_DECISION2_JSON = `{"Id":"test-id2","Name":"What is your other decision?","BallotId":"transaction-id","Options":[{"Id":"b","Name":"B","Attributes":{"image":"/url"}}],"Attributes":{"Key":"Value"}}`
const TEST_DECISION_JSON = `{"Id":"test-id","Name":"What is your decision?","BallotId":"transaction-id","Options":[{"Id":"a","Name":"A","Description":"","Attributes":{"image":"/url"}}],"Attributes":{"Key":"Value"},"ResponsesRequired":1,"RepeatVoteDelaySeconds":0,"Repeatable":false}`
const CREATE_DECISION_JSON_BALLOT2 = `{"Id":"test-id2","Name":"What is your decision?","BallotId":"otherballot","Options":[{"Id":"a","Name":"A","Description":"","Attributes":{"image":"/url"}}],"Attributes":{"Key":"Value"}}`
const CREATE_DECISION_JSON_REQUIRED_2 = `{"Id":"test-id","Name":"What is your decision?","BallotId":"transaction-id","Options":[{"Id":"a","Name":"A","Description":"","Attributes":{"image":"/url"}}],"Attributes":{"Key":"Value"},"ResponsesRequired":2}`
const CREATE_REPEATABLE_DECISION_JSON = `{"Id":"test-id","Name":"What is your decision?","BallotId":"transaction-id","Options":[{"Id":"a","Name":"A","Description":"","Attributes":{"image":"/url"}}],"Attributes":{"Key":"Value"},"ResponsesRequired":1,"RepeatVoteDelaySeconds":100,"Repeatable":true}`


func mockEnv(){
	//makes certificate netvote/VOTER/id return slanders
	os.Setenv("TEST_ENV","1")
}

func unmockEnv(){
	os.Unsetenv("TEST_ENV")
}

func resetTime(){
	os.Unsetenv("TEST_TIME")
}

func to_byte_array(function string, args []string)([][]byte){
	if(len(args) == 2){
		args = append(args, "10")
	}
	result := [][]byte{}

	result = append(result, []byte(function))

	for _,it := range args {
		result = append(result, []byte(it))
	}
	return result
}

func checkQuery(t *testing.T, stub *shim.MockStub, function string, args []string, value string) {
	//b_args := to_byte_array(function, args)
	args = append([]string{function}, args...)

	resp := stub.MockInvoke("QUERYID", to_byte_array(function, args))


	if resp.Status != 200 {
		fmt.Println("Invoke", args, "failed", resp.Message)
		t.FailNow()
	}
	if resp.Payload == nil {
		fmt.Println("Response is nil")
		t.FailNow()
	}
	if string(resp.Payload) != value {
		fmt.Println(string(resp.Payload))
		fmt.Println("State value", string(resp.Payload), "was not", value, "as expected")
		t.FailNow()
	}
}

func checkInvoke(t *testing.T, stub *shim.MockStub, function string, args []string) {
	checkInvokeTX(t, stub, "1", function, args)
}

func checkInvokeTX(t *testing.T, stub *shim.MockStub, transactionId string, function string, args []string) {
	args = append([]string{function}, args...)
	resp := stub.MockInvoke("DOESNTMATTER", to_byte_array(function, args))
	fmt.Println("payload="+string(resp.Payload))
	if resp.Status != 200 {
		fmt.Println("Invoke", args, "failed", resp.Message)
		t.FailNow()
	}
}

func checkInvokeError(t *testing.T, stub *shim.MockStub, function string, args []string, error string) {
	args = append([]string{function}, args...)
	resp := stub.MockInvoke("1", to_byte_array(function, args))
	if resp.Status == 200 {
		fmt.Println("No error was found, but error was expected: "+error)
		t.FailNow()
	}
	if resp.Message != error {

		fmt.Println("Expected: "+error+", Found: "+resp.Message)
		t.FailNow()
	}
}

func checkGone(t *testing.T, stub *shim.MockStub, name string){
	bytes := stub.State[name]
	if bytes != nil {
		fmt.Println("Expected state", name, "to be gone")
		t.FailNow()
	}
}

func checkState(t *testing.T, stub *shim.MockStub, name string, value string) {
	bytes := stub.State[name]
	if bytes == nil {
		fmt.Println("State", name, "failed to get value")
		t.FailNow()
	}
	if string(bytes) != value {
		fmt.Println(string(bytes))
		fmt.Println("State value", name, "was not", value, "as expected")
		t.FailNow()
	}
}

func TestVoteChaincode_Invoke_AddPrivateBallotWithDecisions(t *testing.T) {
	mockEnv()
	scc := new(VoteChaincode)

	stub := shim.NewMockStub("vote", scc)

	stub.MockTransactionStart("test-invoke-add-ballot")

	checkInvokeTX(t, stub, "transaction-id", "add_ballot",
		[]string{`{"Ballot":{"Id":"transaction-id","Name":"Nov 8, 2016","Private": true, "Active":true}, "Decisions":[` + CREATE_DECISION_JSON + `]}`, "10"})

	checkInvokeTX(t, stub,  "transaction-id2", "add_ballot",
		[]string{`{"Ballot":{"Id":"transaction-id2","Name":"Nov 8, 2016", "Active":true}, "Decisions":[`+CREATE_DECISION_JSON+`]}`, "10"})


	checkInvoke(t, stub, "cast_votes", []string{`{"VoterId":"slanders", "BallotId":"transaction-id", "Decisions":[{"DecisionId":"test-id", "Selections": {"a":1}}]}`, "100"})
	checkState(t, stub, "netvote/VOTER/slanders", `{"Id":"slanders","Dimensions":[],"DecisionIdToVoteCount":{"transaction-id":{"test-id":0}},"DecisionTimestamps":{"transaction-id":{"test-id":[100]}},"LastVoteTimestampSeconds":100,"Attributes":null}`)

	// check subsequent assignment has no effect
	checkInvoke(t, stub, "assign_ballot", []string{`{"BallotId":"transaction-id","Voter":{"Id":"slanders","Dimensions":["us","ga","123"]}}`, "100"})
	checkState(t, stub, "netvote/VOTER/slanders", `{"Id":"slanders","Dimensions":[],"DecisionIdToVoteCount":{"transaction-id":{"test-id":0}},"DecisionTimestamps":{"transaction-id":{"test-id":[100]}},"LastVoteTimestampSeconds":100,"Attributes":null}`)


}

func TestVoteChaincode_Invoke_AddBallotWithDecisions(t *testing.T){
	mockEnv()
	scc := new(VoteChaincode)

	stub := shim.NewMockStub("vote", scc)

	stub.MockTransactionStart("test-invoke-add-ballot")

	checkInvokeTX(t, stub,  "transaction-id", "add_ballot",
		[]string{`{"Ballot":{"Id":"transaction-id","Name":"Nov 8, 2016","Active":true}, "Decisions":[`+CREATE_DECISION_JSON+`]}`})

	checkState(t, stub, "netvote/BALLOT/transaction-id", `{"Id":"transaction-id","Name":"Nov 8, 2016","Decisions":["test-id"],"Private":false,"Attributes":null,"Description":"","StartTimeSeconds":0,"EndTimeSeconds":0,"Active":true}`)
	checkState(t, stub, "netvote/DECISION/transaction-id/test-id", TEST_DECISION_JSON)
	checkState(t, stub, "netvote/RESULTS/transaction-id/test-id", `{"Id":"test-id","Results":{}}`)

	checkQuery(t, stub, "get_admin_ballot", []string{`{"Id":"transaction-id"}`}, `{"Ballot":{"Id":"transaction-id","Name":"Nov 8, 2016","Decisions":["test-id"],"Private":false,"Attributes":null,"Description":"","StartTimeSeconds":0,"EndTimeSeconds":0,"Active":true},"Decisions":[{"Id":"test-id","Name":"What is your decision?","BallotId":"transaction-id","Options":[{"Id":"a","Name":"A","Description":"","Attributes":{"image":"/url"}}],"Attributes":{"Key":"Value"},"ResponsesRequired":1,"RepeatVoteDelaySeconds":0,"Repeatable":false}]}`)

	checkInvokeTX(t, stub,  "transaction-id", "delete_ballot",
		[]string{`{"Id":"transaction-id"}`})

	checkGone(t, stub, "netvote/BALLOT/transaction-id")
	checkGone(t, stub, "netvote/DECISION/transaction-id/test-id")
	checkGone(t, stub, "netvote/RESULTS/transaction-id/test-id")
	checkState(t, stub, "netvote/ACCOUNT_BALLOTS/netvote", `{"Id":"netvote","PublicBallotIds":{},"PrivateBallotIds":{}}`)

}

func TestVoteChaincode_Invoke_TestMultipleAllocates(t *testing.T) {
	mockEnv()
	scc := new(VoteChaincode)

	stub := shim.NewMockStub("vote", scc)
	stub.MockTransactionStart("test-invoke-add-decision")

	//setup

	checkInvokeTX(t, stub,  "transaction-id", "add_ballot",
		[]string{`{"Ballot":{"Id":"transaction-id","Name":"Nov 8, 2016","Active":true}, "Decisions":[`+CREATE_DECISION_JSON+`]}`})

	checkInvokeTX(t, stub, "transaction-id", "init_voter", []string{`{"Id":"slanders"}`})

	checkState(t, stub, "netvote/VOTER/slanders", `{"Id":"slanders","Dimensions":[],"DecisionIdToVoteCount":{"transaction-id":{"test-id":1}},"DecisionTimestamps":{"transaction-id":{"test-id":[]}},"LastVoteTimestampSeconds":0,"Attributes":null}`)

	//cast votes
	checkInvoke(t, stub, "cast_votes", []string{`{"VoterId":"slanders", "BallotId":"transaction-id", "Decisions":[{"DecisionId":"test-id", "Selections": {"a":1}}]}`,"100"})
	checkState(t, stub, "netvote/VOTER/slanders", `{"Id":"slanders","Dimensions":[],"DecisionIdToVoteCount":{"transaction-id":{"test-id":0}},"DecisionTimestamps":{"transaction-id":{"test-id":[100]}},"LastVoteTimestampSeconds":100,"Attributes":null}`)

	//try to re-allocate votes, votes should remain at 0 for this decision
	checkInvokeTX(t, stub, "transaction-id", "init_voter", []string{`{"Id":"slanders"}`,"100"})
	checkState(t, stub, "netvote/VOTER/slanders", `{"Id":"slanders","Dimensions":[],"DecisionIdToVoteCount":{"transaction-id":{"test-id":0}},"DecisionTimestamps":{"transaction-id":{"test-id":[100]}},"LastVoteTimestampSeconds":100,"Attributes":null}`)
	resetTime()
}

func TestVoteChaincode_Invoke_AddVoter(t *testing.T) {
	scc := new(VoteChaincode)
	stub := shim.NewMockStub("vote", scc)

	stub.MockTransactionStart("test-invoke-add-voter")

	checkInvoke(t, stub, "add_voter", []string{`{"Id":"voter-id","Dimensions":["us","ga","123"],"DecisionIdToVoteCount":{"ballotId":{"d1":2,"d2":1}}}`})

	checkState(t, stub, "netvote/VOTER/voter-id", `{"Id":"voter-id","Dimensions":["us","ga","123"],"DecisionIdToVoteCount":{"ballotId":{"d1":2,"d2":1}},"DecisionTimestamps":null,"LastVoteTimestampSeconds":0,"Attributes":null}`)

}

func TestVoteChaincode_Invoke_InvalidFunction(t *testing.T) {
	scc := new(VoteChaincode)
	stub := shim.NewMockStub("vote", scc)

	stub.MockTransactionStart("test-invoke-bad-function")

	checkInvokeError(t, stub, "not_real", []string{``}, "Invalid Function: not_real")
}

func TestVoteChaincode_Invoke_CastVote(t *testing.T) {
	mockEnv()
	scc := new(VoteChaincode)

	stub := shim.NewMockStub("vote", scc)

	stub.MockTransactionStart("test-invoke-add-decision")

	checkInvokeTX(t, stub,  "transaction-id", "add_ballot",
		[]string{`{"Ballot":{"Id":"transaction-id","Name":"Nov 8, 2016","Active":true}, "Decisions":[`+CREATE_DECISION_JSON+`]}`})

	checkState(t, stub, "netvote/DECISION/transaction-id/test-id", TEST_DECISION_JSON)
	checkState(t, stub, "netvote/BALLOT/transaction-id", `{"Id":"transaction-id","Name":"Nov 8, 2016","Decisions":["test-id"],"Private":false,"Attributes":null,"Description":"","StartTimeSeconds":0,"EndTimeSeconds":0,"Active":true}`)
	checkInvokeTX(t, stub, "transaction-id", "init_voter", []string{`{"Id":"slanders"}`})
	checkState(t, stub, "netvote/VOTER/slanders", `{"Id":"slanders","Dimensions":[],"DecisionIdToVoteCount":{"transaction-id":{"test-id":1}},"DecisionTimestamps":{"transaction-id":{"test-id":[]}},"LastVoteTimestampSeconds":0,"Attributes":null}`)
	checkState(t, stub, "netvote/ACCOUNT_BALLOTS/netvote", `{"Id":"netvote","PublicBallotIds":{"transaction-id":true},"PrivateBallotIds":{}}`)

	checkQuery(t, stub, "get_ballot", []string{`{"VoterId":"slanders","BallotId":"transaction-id"}`}, `[{"Id":"test-id","Name":"What is your decision?","BallotId":"transaction-id","Options":[{"Id":"a","Name":"A","Description":"","Attributes":{"image":"/url"}}],"Attributes":{"Key":"Value"},"ResponsesRequired":1,"RepeatVoteDelaySeconds":0,"Repeatable":false}]`)


	checkInvoke(t, stub, "cast_votes", []string{`{"VoterId":"slanders", "BallotId": "transaction-id", "Dimensions":["US","GA"], "Decisions":[{"DecisionId":"test-id", "Selections": {"a":1}}]}`,"500"})

	checkState(t, stub, "netvote/VOTER/slanders", `{"Id":"slanders","Dimensions":[],"DecisionIdToVoteCount":{"transaction-id":{"test-id":0}},"DecisionTimestamps":{"transaction-id":{"test-id":[500]}},"LastVoteTimestampSeconds":500,"Attributes":null}`)
	checkState(t, stub, "netvote/RESULTS/transaction-id/test-id", `{"Id":"test-id","Results":{"ALL":{"a":1},"GA":{"a":1},"US":{"a":1}}}`)
}

func TestVoteChaincode_Invoke_CastVoteMultiBallot(t *testing.T) {
	mockEnv()
	scc := new(VoteChaincode)

	stub := shim.NewMockStub("vote", scc)

	stub.MockTransactionStart("test-invoke-add-decision")

	checkInvokeTX(t, stub,  "transaction-id", "add_ballot",
		[]string{`{"Ballot":{"Id":"transaction-id","Name":"Nov 8, 2016","Active":true}, "Decisions":[`+CREATE_DECISION_JSON+`]}`})
	checkInvokeTX(t, stub,  "transaction-id", "add_ballot",
		[]string{`{"Ballot":{"Id":"otherballot","Name":"Nov 8, 2016","Active":true}, "Decisions":[`+CREATE_DECISION_JSON_BALLOT2+`]}`})

	checkState(t, stub, "netvote/DECISION/transaction-id/test-id", TEST_DECISION_JSON)
	checkState(t, stub, "netvote/BALLOT/transaction-id", `{"Id":"transaction-id","Name":"Nov 8, 2016","Decisions":["test-id"],"Private":false,"Attributes":null,"Description":"","StartTimeSeconds":0,"EndTimeSeconds":0,"Active":true}`)
	checkInvokeTX(t, stub, "transaction-id", "init_voter", []string{`{"Id":"slanders"}`})
	checkState(t, stub, "netvote/VOTER/slanders", `{"Id":"slanders","Dimensions":[],"DecisionIdToVoteCount":{"otherballot":{"test-id2":1},"transaction-id":{"test-id":1}},"DecisionTimestamps":{"otherballot":{"test-id2":[]},"transaction-id":{"test-id":[]}},"LastVoteTimestampSeconds":0,"Attributes":null}`)
	checkState(t, stub, "netvote/ACCOUNT_BALLOTS/netvote", `{"Id":"netvote","PublicBallotIds":{"otherballot":true,"transaction-id":true},"PrivateBallotIds":{}}`)

	checkQuery(t, stub, "get_ballot", []string{`{"VoterId":"slanders","BallotId":"transaction-id"}`}, `[{"Id":"test-id","Name":"What is your decision?","BallotId":"transaction-id","Options":[{"Id":"a","Name":"A","Description":"","Attributes":{"image":"/url"}}],"Attributes":{"Key":"Value"},"ResponsesRequired":1,"RepeatVoteDelaySeconds":0,"Repeatable":false}]`)
	checkQuery(t, stub, "get_ballot", []string{`{"VoterId":"slanders","BallotId":"otherballot"}`}, `[{"Id":"test-id2","Name":"What is your decision?","BallotId":"otherballot","Options":[{"Id":"a","Name":"A","Description":"","Attributes":{"image":"/url"}}],"Attributes":{"Key":"Value"},"ResponsesRequired":1,"RepeatVoteDelaySeconds":0,"Repeatable":false}]`)

	checkQuery(t, stub, "get_voter_ballots", []string{`{"Id":"slanders"}`}, `[{"Id":"otherballot","Name":"Nov 8, 2016","Decisions":["test-id2"],"Private":false,"Attributes":null,"Description":"","StartTimeSeconds":0,"EndTimeSeconds":0,"Active":true},{"Id":"transaction-id","Name":"Nov 8, 2016","Decisions":["test-id"],"Private":false,"Attributes":null,"Description":"","StartTimeSeconds":0,"EndTimeSeconds":0,"Active":true}]`)


	checkInvoke(t, stub, "cast_votes", []string{`{"VoterId":"slanders", "BallotId":"transaction-id", "Decisions":[{"DecisionId":"test-id", "Selections": {"a":1}}]}`,"500"})
	checkState(t, stub, "netvote/VOTER/slanders", `{"Id":"slanders","Dimensions":[],"DecisionIdToVoteCount":{"otherballot":{"test-id2":1},"transaction-id":{"test-id":0}},"DecisionTimestamps":{"otherballot":{"test-id2":[]},"transaction-id":{"test-id":[500]}},"LastVoteTimestampSeconds":500,"Attributes":null}`)
	checkState(t, stub, "netvote/RESULTS/transaction-id/test-id", `{"Id":"test-id","Results":{"ALL":{"a":1}}}`)
	checkState(t, stub, "netvote/RESULTS/otherballot/test-id2", `{"Id":"test-id2","Results":{}}`)

	checkQuery(t, stub, "get_voter_ballots", []string{`{"Id":"slanders"}`}, `[{"Id":"otherballot","Name":"Nov 8, 2016","Decisions":["test-id2"],"Private":false,"Attributes":null,"Description":"","StartTimeSeconds":0,"EndTimeSeconds":0,"Active":true}]`)

	checkInvoke(t, stub, "cast_votes", []string{`{"VoterId":"slanders", "BallotId":"otherballot", "Decisions":[{"DecisionId":"test-id2", "Selections": {"a":1}}]}`,"1000"})
	checkState(t, stub, "netvote/VOTER/slanders", `{"Id":"slanders","Dimensions":[],"DecisionIdToVoteCount":{"otherballot":{"test-id2":0},"transaction-id":{"test-id":0}},"DecisionTimestamps":{"otherballot":{"test-id2":[1000]},"transaction-id":{"test-id":[500]}},"LastVoteTimestampSeconds":1000,"Attributes":null}`)
	checkState(t, stub, "netvote/RESULTS/transaction-id/test-id", `{"Id":"test-id","Results":{"ALL":{"a":1}}}`)
	checkState(t, stub, "netvote/RESULTS/otherballot/test-id2", `{"Id":"test-id2","Results":{"ALL":{"a":1}}}`)

	checkQuery(t, stub, "get_voter_ballots", []string{`{"Id":"slanders"}`}, `[]`)

}

func TestVoteChaincode_Invoke_CastRepeatableVote(t *testing.T){
	mockEnv()
	scc := new(VoteChaincode)

	stub := shim.NewMockStub("vote", scc)

	stub.MockTransactionStart("test-invoke-add-decision")


	checkInvokeTX(t, stub,  "transaction-id", "add_ballot",
		[]string{`{"Ballot":{"Id":"transaction-id","Name":"","Active":true}, "Decisions":[`+CREATE_REPEATABLE_DECISION_JSON+`]}`})

	checkState(t, stub, "netvote/DECISION/transaction-id/test-id", CREATE_REPEATABLE_DECISION_JSON)
	checkState(t, stub, "netvote/BALLOT/transaction-id", `{"Id":"transaction-id","Name":"","Decisions":["test-id"],"Private":false,"Attributes":null,"Description":"","StartTimeSeconds":0,"EndTimeSeconds":0,"Active":true}`)
	checkInvokeTX(t, stub, "transaction-id", "init_voter", []string{`{"Id":"slanders"}`})
	checkState(t, stub, "netvote/VOTER/slanders", `{"Id":"slanders","Dimensions":[],"DecisionIdToVoteCount":{"transaction-id":{"test-id":1}},"DecisionTimestamps":{"transaction-id":{"test-id":[]}},"LastVoteTimestampSeconds":0,"Attributes":null}`)
	checkState(t, stub, "netvote/ACCOUNT_BALLOTS/netvote", `{"Id":"netvote","PublicBallotIds":{"transaction-id":true},"PrivateBallotIds":{}}`)
	checkInvoke(t, stub, "cast_votes", []string{`{"VoterId":"slanders", "BallotId":"transaction-id", "Decisions":[{"DecisionId":"test-id", "Selections": {"a":1}}]}`,"500"})

	checkState(t, stub, "netvote/VOTER/slanders", `{"Id":"slanders","Dimensions":[],"DecisionIdToVoteCount":{"transaction-id":{"test-id":1}},"DecisionTimestamps":{"transaction-id":{"test-id":[500]}},"LastVoteTimestampSeconds":500,"Attributes":null}`)
	checkState(t, stub, "netvote/RESULTS/transaction-id/test-id", `{"Id":"test-id","Results":{"ALL":{"a":1}}}`)

	checkInvoke(t, stub, "assign_ballot", []string{`{"BallotId":"transaction-id","Voter":{"Id":"slanders","Dimensions":[]}}`})

	checkInvokeError(t, stub, "cast_votes", []string{`{"VoterId":"slanders","BallotId":"transaction-id", "Decisions":[{"DecisionId":"test-id", "Selections": {"a":1}}]}`}, "Already voted this period")
	checkInvoke(t, stub, "cast_votes", []string{`{"VoterId":"slanders","BallotId":"transaction-id", "Decisions":[{"DecisionId":"test-id", "Selections": {"a":1}}]}`,"1500"})
	checkState(t, stub, "netvote/RESULTS/transaction-id/test-id", `{"Id":"test-id","Results":{"ALL":{"a":2}}}`)

	checkState(t, stub, "netvote/VOTER/slanders", `{"Id":"slanders","Dimensions":[],"DecisionIdToVoteCount":{"transaction-id":{"test-id":1}},"DecisionTimestamps":{"transaction-id":{"test-id":[500,1500]}},"LastVoteTimestampSeconds":1500,"Attributes":null}`)

}

func TestVoteChaincode_Invoke_QueryBallotResults(t *testing.T){
	mockEnv()
	scc := new(VoteChaincode)

	stub := shim.NewMockStub("vote", scc)

	stub.MockTransactionStart("test-invoke-add-ballot")

	checkInvokeTX(t, stub,  "transaction-id", "add_ballot",
		[]string{`{"Ballot":{"Id":"transaction-id","Name":"Nov 8, 2016","Active":true}, "Decisions":[`+CREATE_DECISION_JSON+`,`+CREATE_DECISION2_JSON+`]}`})

	checkState(t, stub, "netvote/BALLOT/transaction-id", `{"Id":"transaction-id","Name":"Nov 8, 2016","Decisions":["test-id","test-id2"],"Private":false,"Attributes":null,"Description":"","StartTimeSeconds":0,"EndTimeSeconds":0,"Active":true}`)
	checkState(t, stub, "netvote/DECISION/transaction-id/test-id", TEST_DECISION_JSON)
	checkState(t, stub, "netvote/RESULTS/transaction-id/test-id", `{"Id":"test-id","Results":{}}`)

	checkInvokeTX(t, stub, "transaction-id", "init_voter", []string{`{"Id":"slanders"}`})

	checkInvoke(t, stub, "cast_votes", []string{`{"VoterId":"slanders", "BallotId":"transaction-id", "Decisions":[{"DecisionId":"test-id", "Selections": {"a":1}}]}`})

	checkState(t, stub, "netvote/RESULTS/transaction-id/test-id", `{"Id":"test-id","Results":{"ALL":{"a":1}}}`)

	checkQuery(t, stub, "get_ballot_results", []string{`{"Id":"transaction-id"}`}, `{"Id":"transaction-id","Results":{"test-id":{"Id":"test-id","Results":{"ALL":{"a":1}}},"test-id2":{"Id":"test-id2","Results":{}}}}`)
}

func TestVoteChaincode_Invoke_ValidateCastTooMany(t *testing.T) {
	scc := new(VoteChaincode)
	stub := shim.NewMockStub("vote", scc)

	stub.MockTransactionStart("test-invoke-cast-vote")

	checkInvokeTX(t, stub,  "transaction-id", "add_ballot",
		[]string{`{"Ballot":{"Id":"transaction-id","Name":"Nov 8, 2016","Active":true}, "Decisions":[`+TEST_DECISION_JSON+`]}`})

	checkInvokeTX(t, stub, "transaction-id", "init_voter", []string{`{"Id":"slanders"}`})

	checkInvokeError(t, stub, "cast_votes", []string{`{"VoterId":"slanders", "BallotId":"transaction-id", "Decisions":[{"DecisionId":"test-id", "Selections": {"a":2}}]}`}, "Values must add up to exactly ResponsesRequired")
}

func TestVoteChaincode_Invoke_ValidateInvalidOption(t *testing.T) {
	scc := new(VoteChaincode)
	stub := shim.NewMockStub("vote", scc)

	stub.MockTransactionStart("test-invoke-cast-vote")
	checkInvokeTX(t, stub,  "transaction-id", "add_ballot",
		[]string{`{"Ballot":{"Id":"transaction-id","Name":"Nov 8, 2016","Active":true}, "Decisions":[`+TEST_DECISION_JSON+`]}`})

	checkInvokeTX(t, stub, "transaction-id", "init_voter", []string{`{"Id":"slanders"}`})

	checkInvokeError(t, stub, "cast_votes", []string{`{"VoterId":"slanders", "BallotId":"transaction-id", "Decisions":[{"DecisionId":"test-id", "Selections": {"c":1}}]}`}, "Invalid option: c")
}

func TestVoteChaincode_Invoke_ValidateNotActive(t *testing.T) {
	scc := new(VoteChaincode)
	stub := shim.NewMockStub("vote", scc)

	stub.MockTransactionStart("test-invoke-cast-vote")

	checkInvokeTX(t, stub,  "transaction-id", "add_ballot",
		[]string{`{"Ballot":{"Id":"transaction-id","Name":"Nov 8, 2016","Active":false}, "Decisions":[`+TEST_DECISION_JSON+`]}`})


	checkInvokeTX(t, stub, "transaction-id", "init_voter", []string{`{"Id":"slanders"}`})

	checkInvokeError(t, stub, "cast_votes", []string{`{"VoterId":"slanders", "BallotId":"transaction-id", "Decisions":[{"DecisionId":"test-id", "Selections": {"a":1}}]}`}, "This ballot is not active")
}

func TestVoteChaincode_Invoke_ValidateCastTooFew(t *testing.T) {
	scc := new(VoteChaincode)
	stub := shim.NewMockStub("vote", scc)

	stub.MockTransactionStart("test-invoke-cast-vote")

	checkInvokeTX(t, stub,  "transaction-id", "add_ballot",
		[]string{`{"Ballot":{"Id":"transaction-id","Name":"Nov 8, 2016","Active":true}, "Decisions":[`+CREATE_DECISION_JSON_REQUIRED_2+`]}`})


	checkInvokeTX(t, stub, "transaction-id", "init_voter", []string{`{"Id":"slanders"}`})

	checkInvokeError(t, stub, "cast_votes", []string{`{"VoterId":"slanders", "BallotId":"transaction-id", "Decisions":[{"DecisionId":"test-id", "Selections": {"a":1}}]}`}, "All selections must be made")
}

func TestVoteChaincode_Invoke_InitVoterWithAttributes(t *testing.T) {
	scc := new(VoteChaincode)
	stub := shim.NewMockStub("vote", scc)

	stub.MockTransactionStart("test-invoke-cast-vote")

	checkInvokeTX(t, stub,  "transaction-id", "add_ballot",
		[]string{`{"Ballot":{"Id":"transaction-id","Name":"Nov 8, 2016"}, "Decisions":[`+CREATE_DECISION_JSON+`]}`})


	checkInvokeTX(t, stub, "transaction-id", "init_voter", []string{`{"Id":"slanders","Attributes":{"key":"val"}}`})
	checkState(t, stub, "netvote/VOTER/slanders", `{"Id":"slanders","Dimensions":[],"DecisionIdToVoteCount":{"transaction-id":{"test-id":1}},"DecisionTimestamps":{"transaction-id":{"test-id":[]}},"LastVoteTimestampSeconds":0,"Attributes":{"key":"val"}}`)
}