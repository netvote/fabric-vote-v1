package main

import (
	"errors"
	"fmt"
	"github.com/hyperledger/fabric/core/chaincode/shim"
	pb "github.com/hyperledger/fabric/protos/peer"
	"encoding/json"
	"time"
	"strconv"
	"sort"
)

//TODO: if blockchains are multi-elections, will need scoping by 'election'
//TODO: add time windows for ballots/decisions? to allow valid voting periods

// voter dimension (defaults)
const DIMENSION_ALL = "ALL"
const ATTRIBUTE_ROLE = "role"

const ROLE_ADMIN = "admin"

// function names
const QUERY_GET_ADMIN_BALLOT = "get_admin_ballot";

const FUNC_ADD_VOTER = "add_voter"
const FUNC_ADD_BALLOT = "add_ballot"
const FUNC_DELETE_BALLOT = "delete_ballot"
const FUNC_CAST_VOTES = "cast_votes"
const FUNC_INIT_VOTER = "init_voter"
const FUNC_ASSIGN_BALLOT = "assign_ballot"
const QUERY_GET_BALLOT_RESULTS = "get_ballot_results"
const QUERY_GET_BALLOT = "get_ballot"
const QUERY_GET_VOTER_BALLOTS = "get_voter_ballots"
const QUERY_GET_DECISIONS = "get_decisions"
const QUERY_GET_ACCOUNT_BALLOTS = "get_account_ballots"


type VoteChaincode struct {
}

type Option struct {
	Id string
	Name string
	Description string
	Attributes map[string]string
}

type Decision struct {
	Id                string
	Name              string
	BallotId          string
	Description	  string
	Options           []Option
	Attributes map[string]string
	ResponsesRequired int
	RepeatVoteDelaySeconds int
	Repeatable        bool
}

type Ballot struct{
	Id string
	Name string
	Decisions []string
	Private bool
	Attributes map[string]string
	Description string
	StartTimeSeconds int
	EndTimeSeconds int
	Active bool
}

func (t *Ballot) ActiveElection(now int)(bool){
	return t.Active || t.ActiveDates(now)
}

func (t *Ballot) ActiveDates(now int)(bool){
	begins_in_past := (t.StartTimeSeconds <= now)
	ends_in_future := (t.EndTimeSeconds >= now)
	return begins_in_past && ends_in_future
}


type BallotDecisions struct{
	Ballot Ballot
	Decisions []Decision
}

type BallotResults struct {
	Id string
	Results map[string]DecisionResults
}

type DecisionResults struct{
	Id string
	Results map[string]map[string]int
}


type Voter struct {
	Id string
	Dimensions []string
	DecisionIdToVoteCount map[string]map[string]int
	DecisionTimestamps map[string]map[string][]int;
	LastVoteTimestampSeconds int
	Attributes map[string]string
}

type AccountBallots struct{
	Id string
	PublicBallotIds map[string]bool
	PrivateBallotIds map[string]bool
}

type BallotAssignment struct {
	BallotId string
	Voter Voter
}

type Vote struct {
	BallotId string
	VoterId string
	Decisions []VoterDecision
	Dimensions []string
	VoterAttributes map[string]string
}

type VoterDecision struct {
	DecisionId string
	Selections map[string]int
	Reasons map[string]map[string]string
	Attributes map[string]string
}

//NOTE: must match structure in eventlistener.go
type VoteEvent struct {
	Ballot BallotDecisions
	Dimensions []string
	VoterAttributes map[string]string
	VoteDecisions []VoterDecision
	AccountId string
	BallotResults BallotResults
}

func stringInSlice(a string, list []Option) bool {
	for _, b := range list {
		if b.Id == a {
			return true
		}
	}
	return false
}

func doPanic(statusCode int, message string){
	jsonErr, _ := json.Marshal(getResponse(statusCode, message))
	panic(string(jsonErr))
}

func getResponse(statusCode int, message string)(pb.Response){
	return pb.Response {
		Status: int32(statusCode),
		Message: fmt.Sprintf(`{"Code":%s,"Message":"%s"}`, strconv.Itoa(statusCode), message)}
}

func validate(stateDao StateDAO, vote Vote, voter Voter){

	printJson("validate voter", voter)
	printJson("validate vote", vote)
	if(vote.BallotId == ""){
		//TODO: for now, this is required
		doPanic(400, "BallotId is required")
	}

	ballot := stateDao.GetBallot(vote.BallotId)

	if(!ballot.ActiveElection(stateDao.timeInSeconds)){
		doPanic(403, "This ballot is not active")
	}

	for _, decision := range vote.Decisions {
		d := stateDao.GetDecision(vote.BallotId, decision.DecisionId)

		if(voter.DecisionIdToVoteCount == nil || voter.DecisionIdToVoteCount[vote.BallotId][decision.DecisionId] == 0) {
			doPanic(403, "This voter has no votes")
		}
		if(d.ResponsesRequired != len(decision.Selections)){
			doPanic(400, "All selections must be made")
		}
		if(d.Repeatable){
			if(alreadyVoted(stateDao, voter, d)){
				doPanic(403, "Already voted this period")
			}
		}
		var total int= 0
		for _, sel := range decision.Selections{
			total += sel
		}
		if(total != voter.DecisionIdToVoteCount[vote.BallotId][decision.DecisionId]){
			printJson("DecisionId", decision.DecisionId)
			doPanic(400, "Values must add up to exactly ResponsesRequired")
		}

		for k,_ := range decision.Selections {
			if(!stringInSlice(k, d.Options)){
				doPanic(400, "Invalid option: "+k)
			}
		}
	}
}

func alreadyVoted(stateDao StateDAO, voter Voter, decision Decision)(bool){
	decisionHistory :=  voter.DecisionTimestamps[decision.BallotId][decision.Id];
	votedBefore := len(decisionHistory) > 0

	if(votedBefore && decision.Repeatable){
		votedBefore = (decisionHistory[len(decisionHistory)-1] > (stateDao.timeInSeconds-decision.RepeatVoteDelaySeconds))
	}
	return votedBefore
}

func addBallotDecisionsToVoter(stateDao StateDAO, ballot Ballot, voter *Voter, save bool){
	for _, decisionId := range ballot.Decisions {
		decision := stateDao.GetDecision(ballot.Id,decisionId)
		addDecisionToVoter(ballot.Id, voter, decision)
	}
	if(save) {
		printJson("saving voter", voter)
		stateDao.SaveVoter(*voter)
	}
}

func addDecisionToVoter(ballotId string, voter *Voter, decision Decision){
	if(voter.DecisionIdToVoteCount == nil){
		voter.DecisionIdToVoteCount = make(map[string]map[string]int)
	}
	if(voter.DecisionIdToVoteCount[ballotId] == nil){
		voter.DecisionIdToVoteCount[ballotId] = make(map[string]int)
	}
	if(voter.DecisionTimestamps == nil){
		voter.DecisionTimestamps = make(map[string]map[string][]int)
	}
	if(voter.DecisionTimestamps[ballotId] == nil){
		voter.DecisionTimestamps[ballotId] = make(map[string][]int)
	}
	if(voter.DecisionTimestamps[ballotId][decision.Id] == nil){
		voter.DecisionTimestamps[ballotId][decision.Id] = make([]int, 0)
	}
	if _, exists := voter.DecisionIdToVoteCount[ballotId][decision.Id]; exists {
		//already allocated for this, skip
	}else{
		voter.DecisionIdToVoteCount[ballotId][decision.Id] = decision.ResponsesRequired
	}
}

func addBallot(stateDao StateDAO, ballotDecisions BallotDecisions) (Ballot){
	ballot := ballotDecisions.Ballot
	ballot.Decisions = []string{}
	if(ballot.Id == ""){
		doPanic(400, "ballot id is required")
	}

	for _, decision := range ballotDecisions.Decisions {
		log("adding decision: "+decision.Name)
		decision.BallotId = ballot.Id
		decision = addDecisionToChain(stateDao, decision)
		ballot.Decisions = append(ballot.Decisions, decision.Id)
	}

	stateDao.SaveBallot(ballot)
	return ballot
}

func addDecisionToBallot(stateDao StateDAO, ballotId string, decisionId string){
	ballot := stateDao.GetBallot(ballotId)
	if(ballot.Id == ""){
		ballot = Ballot{Id: ballotId, Decisions: []string{decisionId}}
		stateDao.SaveBallot(ballot)
	}
}

func log(message string){
	fmt.Printf(time.Now().String()+" - NETVOTE: %s\n", message)
}

func getDimensionsForVote(voter Voter, vote Vote)([]string){
	dimensions := make([]string, 0)
	dimension_map := make(map[string]bool)
	if(voter.Dimensions != nil){
		for _,i := range voter.Dimensions {
			dimension_map[i] = true
		}
	}
	if(vote.Dimensions != nil){
		for _,i := range vote.Dimensions {
			dimension_map[i] = true
		}
	}
	for k,_ := range dimension_map{
		dimensions = append(dimensions, k)
	}
	return dimensions
}

func getAttributesForVote(voter Voter, vote Vote)(map[string]string){
	attributes := make(map[string]string)
	if(voter.Attributes != nil) {
		for k, v := range voter.Attributes {
			attributes[k] = v
		}
	}

	if(vote.VoterAttributes != nil) {
		for k, v := range vote.VoterAttributes {
			attributes[k] = v
		}
	}
	return attributes
}

func initializeVoterFromVote(stateDao StateDAO, vote Vote)(Voter){
	voter :=  lazyInitVoter(stateDao, Voter{ Id: vote.VoterId })
	ballot := stateDao.GetBallot(vote.BallotId)
	addBallotDecisionsToVoter(stateDao, ballot, &voter, true)
	return voter
}

func castVote(stateDao StateDAO, vote Vote){
	voter := initializeVoterFromVote(stateDao, vote)
	validate(stateDao, vote, voter)
	results_array := make([]DecisionResults, 0)

	dimensions := getDimensionsForVote(voter, vote)
	attributes := getAttributesForVote(voter, vote)

	now := stateDao.timeInSeconds

	resultsMap := make(map[string]DecisionResults)

	for _, voter_decision := range vote.Decisions {

		decisionResults := stateDao.GetDecisionResults(vote.BallotId, voter_decision.DecisionId)
		decision := stateDao.GetDecision(vote.BallotId, voter_decision.DecisionId)

		for selection, vote_count := range voter_decision.Selections {
			if(nil == decisionResults.Results[DIMENSION_ALL]){
				decisionResults.Results[DIMENSION_ALL] = map[string]int{selection: 0}
			}

			//cast vote for this decision
			decisionResults.Results[DIMENSION_ALL][selection] += vote_count
			//if not repeatable, remove votes from voter
			if(!decision.Repeatable){
				voter.DecisionIdToVoteCount[vote.BallotId][voter_decision.DecisionId] -= vote_count
			}

			for _, dimension := range dimensions {
				if(nil == decisionResults.Results[dimension]){
					decisionResults.Results[dimension] = map[string]int{selection: 0}
				}
				decisionResults.Results[dimension][selection] += vote_count
			}
		}
		resultsMap[voter_decision.DecisionId] = decisionResults;
		results_array = append(results_array, decisionResults)
		voter.DecisionTimestamps[vote.BallotId][voter_decision.DecisionId] = append(voter.DecisionTimestamps[vote.BallotId][voter_decision.DecisionId], now)
	}
	for _, d := range results_array {
		stateDao.SaveDecisionResults(vote.BallotId, d)
	}
	voter.LastVoteTimestampSeconds = now;
	stateDao.SaveVoter(voter)

	ballot := stateDao.GetBallotDecisions(vote.BallotId)
	ballotResults := BallotResults{Id: ballot.Ballot.Id, Results: resultsMap}

	voteEvent := VoteEvent{
			Ballot: ballot,
			Dimensions: dimensions,
			VoteDecisions: vote.Decisions,
			VoterAttributes: attributes,
			BallotResults: ballotResults,
	}

	stateDao.setVoteEvent(voteEvent)
}

func hasRole(stub shim.ChaincodeStubInterface, role string) (bool){
	return true;
}

func addDecisionToChain(stateDao StateDAO, decision Decision) (Decision){
	if(decision.ResponsesRequired == 0) {
		decision.ResponsesRequired = 1
	}
	if(decision.BallotId == ""){
		doPanic(400, "ballotId is required for decision")
	}
	if(decision.Id == ""){
		doPanic(400, "Id is required for decision")
	}
	results := DecisionResults { Id: decision.Id, Results: make(map[string]map[string]int)}
	stateDao.SaveDecision(decision)
	stateDao.SaveDecisionResults(decision.BallotId, results)
	return decision
}

func addDecision(stateDao StateDAO, decision Decision){
	addDecisionToChain(stateDao, decision)
	if(decision.BallotId != ""){
		addDecisionToBallot(stateDao, decision.BallotId, decision.Id)
	}
}

func addVoter(stateDao StateDAO, voter Voter){
	if(voter.DecisionIdToVoteCount == nil){
		voter.DecisionIdToVoteCount = make(map[string]map[string]int)
	}
	if(voter.Dimensions == nil){
		voter.Dimensions = []string{}
	}
	stateDao.SaveVoter(voter)
}

func parseArg(arg string, value interface{}){
	var arg_bytes = []byte(arg)
	if err := json.Unmarshal(arg_bytes, &value); err != nil {
		doPanic(500, "error parsing arg: "+arg)
	}
}

func lazyInitVoter(stateDao StateDAO, voter Voter)(Voter){
	v := stateDao.GetVoter(voter.Id)
	if(v.Id == ""){
		addVoter(stateDao, voter)
		v = stateDao.GetVoter(voter.Id)
	}
	return v
}

func allocateVotesToVoter(stateDao StateDAO, voter Voter)([]Decision){
	accountBallots := stateDao.GetAccountBallots()
	var result = make([]Decision, 0)
	for ballotId := range accountBallots.PublicBallotIds {
		ballot := stateDao.GetBallot(ballotId)
		addBallotDecisionsToVoter(stateDao, ballot, &voter, false)
	}
	stateDao.SaveVoter(voter)
	return result
}

func printJson(msg string, value interface{}){
	result, _:=  json.Marshal(value)
	log(msg+":"+string(result))
}

func handleInvoke(stub shim.ChaincodeStubInterface, function string, args []string) (resp pb.Response){
	var err error
	var result []byte
	defer func() {
		if r := recover(); r != nil {
			err = errors.New(fmt.Sprintf("%v", r))
			fmt.Printf("error: %v\n",err)
			parseArg(err.Error(), &resp)
		}
	}()

	if(len(args) < 2){
		doPanic(400, "both payload, and timestampInSeconds are required")
	}
	timeInSeconds, er := strconv.Atoi(args[1])
	if(er != nil){
		doPanic(400, "error converting time in seconds from arg[1]")
	}

	stateDao := StateDAO{Stub: stub, timeInSeconds: timeInSeconds}

	switch function {
		// INVOKE
		case FUNC_ADD_BALLOT:
			var ballotDecisions BallotDecisions
			parseArg(args[0], &ballotDecisions)
			ballot := addBallot(stateDao, ballotDecisions)
			result, err = json.Marshal(ballot)
		case FUNC_DELETE_BALLOT:
			var ballot_payload Ballot
			parseArg(args[0], &ballot_payload)

			ballot := stateDao.GetBallot(ballot_payload.Id)
			for _, decisionId := range ballot.Decisions{
				stateDao.DeleteDecision(ballot.Id, decisionId)
			}
			stateDao.DeleteBallot(ballot.Id)
			result = []byte(ballot.Id+" deleted")
		case FUNC_ADD_VOTER:
			var voter Voter
			parseArg(args[0], &voter)
			addVoter(stateDao, voter)
		case FUNC_INIT_VOTER:
			var voter Voter
			parseArg(args[0], &voter)
			voter = lazyInitVoter(stateDao, voter)
			allocateVotesToVoter(stateDao, voter)
		case FUNC_ASSIGN_BALLOT:
			var ballotAssignment BallotAssignment
			parseArg(args[0], &ballotAssignment)
			voter := lazyInitVoter(stateDao, ballotAssignment.Voter)
			ballot := stateDao.GetBallot(ballotAssignment.BallotId)
			addBallotDecisionsToVoter(stateDao, ballot, &voter, true)
		case FUNC_CAST_VOTES:
			var vote Vote
			parseArg(args[0], &vote)
			castVote(stateDao, vote)
		case QUERY_GET_BALLOT_RESULTS:
			var ballotPayload Ballot
			parseArg(args[0], &ballotPayload)
			ballotResults := getBallotResults(stateDao, ballotPayload.Id)
			result, err = json.Marshal(ballotResults)
		case QUERY_GET_DECISIONS:
			var vote_obj Vote
			parseArg(args[0], &vote_obj)
			voter := stateDao.GetVoter(vote_obj.VoterId)
			result, err = json.Marshal(getActiveDecisions(stateDao, voter))
		case QUERY_GET_ACCOUNT_BALLOTS:
			result, err = json.Marshal(stateDao.GetAccountBallots())
		case QUERY_GET_VOTER_BALLOTS:
			var voter_obj Voter
			parseArg(args[0], &voter_obj)
			result, err = json.Marshal(getVoterBallots(stateDao, voter_obj.Id))
		case QUERY_GET_BALLOT:
			var vote_obj Vote
			parseArg(args[0], &vote_obj)
			result, err = json.Marshal(getVoterBallotDecisions(stateDao, vote_obj.VoterId, vote_obj.BallotId))
		case QUERY_GET_ADMIN_BALLOT:
			var ballot_obj Ballot
			parseArg(args[0], &ballot_obj)
			result, err = json.Marshal(stateDao.GetBallotDecisions(ballot_obj.Id))
		default:
			doPanic(400, "Invalid Function: "+function)
	}
	if(result == nil){
		result, err = json.Marshal(getResponse(200, stateDao.Stub.GetTxID()))
	}
	return shim.Success(result)

}

func getActiveDecisions(stateDao StateDAO, voter Voter)([]Decision){
	result := make([]Decision, 0)
	for ballotId,_ := range voter.DecisionIdToVoteCount {
		decisionIdMap := voter.DecisionIdToVoteCount[ballotId]
		for decisionId, _ := range decisionIdMap {
			if (decisionIdMap[decisionId] > 0) {
				decision := stateDao.GetDecision(ballotId, decisionId)
				if (!decision.Repeatable || !alreadyVoted(stateDao, voter, decision)) {
					result = append(result, decision)
				}
			}
		}
	}
	return result
}

func getBallotResults(stateDao StateDAO, ballotId string) BallotResults{
	ballot := stateDao.GetBallot(ballotId)

	resultsMap := make(map[string]DecisionResults)
	for _, decisionId := range ballot.Decisions{
		resultsMap[decisionId] = stateDao.GetDecisionResults(ballot.Id, decisionId)
	}

	return BallotResults { Id: ballot.Id, Results: resultsMap }
}

func getVoterBallots(stateDao StateDAO, voterId string) []Ballot{
	if (voterId == "") {
		doPanic(400, "VoterId and BallotId are required")
	}
	voter := stateDao.GetVoter(voterId)
	active_decisions := getActiveDecisions(stateDao, voter)

	ballotIdMap := make(map[string]bool)
	for _, d := range active_decisions {
		ballotIdMap[d.BallotId] = true
	}

	ballotIds := make([]string, 0)
	for ballotId, _ := range ballotIdMap {
		ballotIds = append(ballotIds, ballotId)
	}
	sort.Strings(ballotIds) //or else order will be non-deterministic

	ballots := make([]Ballot, 0)
	for _, ballotId := range ballotIds {
		ballots = append(ballots, stateDao.GetBallot(ballotId))
	}
	return ballots
}

func getVoterBallotDecisions(stateDao StateDAO, voterId string, ballotId string) []Decision{
	if(ballotId == "" || voterId == ""){
		doPanic(400, "VoterId and BallotId are required")
	}
	voter := stateDao.GetVoter(voterId)
	decisions := make([]Decision,0)
	active_decisions := getActiveDecisions(stateDao, voter)

	for _, d := range active_decisions{
		if(d.BallotId == ballotId) {
			decisions = append(decisions, d)
		}
	}
	return decisions
}



// CHAINCODE INTERFACE METHODS

func (t *VoteChaincode) Invoke(stub shim.ChaincodeStubInterface) pb.Response {
	//function, args := stub.GetFunctionAndParameters()
	_, args := stub.GetFunctionAndParameters()
	return handleInvoke(stub, args[0], args[1:])
}

func (t *VoteChaincode) Init(stub shim.ChaincodeStubInterface) pb.Response  {
	log("Init called")
	return shim.Success(nil)
}

func main() {
	err := shim.Start(new(VoteChaincode))
	if err != nil {
		fmt.Printf("Error starting chaincode: %s", err)
	}
}


// This class contains the accessors for getting/putting state from the blockchian
type StateDAO struct{
	Stub shim.ChaincodeStubInterface
	timeInSeconds int
}

//object types
const TYPE_VOTER = "VOTER"
const TYPE_DECISION = "DECISION"
const TYPE_RESULTS = "RESULTS"
const TYPE_BALLOT = "BALLOT"
const TYPE_ACCOUNT_BALLOTS = "ACCOUNT_BALLOTS"
const ATTRIBUTE_ACCOUNT_ID = "account_id"

func (t *StateDAO) setVoteEvent(voteEvent VoteEvent){
	voteEvent.AccountId = t.getAccountId()
	var json_bytes, err = json.Marshal(voteEvent)
	if err != nil {
		doPanic(500, "Invalid JSON while setting event")
	}
	printJson("EVENT",voteEvent)
	t.Stub.SetEvent("VOTE", json_bytes)
}

func (t *StateDAO) getKey(objectType string, objectId string) (string){
	return t.getAccountId()+"/"+objectType+"/"+objectId
}

func (t *StateDAO) getAccountId()(string){
	//testing hack because it's tricky to mock ReadCertAttribute - hardcoded to limit risk
	/*if(os.Getenv("TEST_ENV") != ""){
		return "netvote"
	}
	result, err := impl.NewAccessControlShim(t.Stub).ReadCertAttribute(ATTRIBUTE_ACCOUNT_ID)

	if(err != nil){
		doPanic("error extracting accountId: "+err.Error())
	}
	return string(result)*/
	return "netvote"
}

func (t *StateDAO) deleteState(objectType string, id string){
	err := t.Stub.DelState(t.getKey(objectType, id))
	if(err != nil){
		doPanic(500, "error deleting "+objectType+" id:"+id)
	}
}

func (t *StateDAO) getState(objectType string, id string, value interface{}){
	config, err := t.Stub.GetState(t.getKey(objectType, id))
	if(err != nil){
		doPanic(500, "error getting "+objectType+" id:"+id)
	}
	json.Unmarshal(config, &value)
}

func (t *StateDAO) GetDecision(ballotId string, decisionId string) (Decision){
	var d Decision
	t.getState(TYPE_DECISION, ballotId+"/"+decisionId, &d)
	return d
}

func (t *StateDAO) GetDecisionResults(ballotId string, decisionId string) (DecisionResults){
	var d DecisionResults
	t.getState(TYPE_RESULTS, ballotId+"/"+decisionId, &d)
	return d
}

func (t *StateDAO) GetVoter(voterId string) (Voter) {
	var v Voter
	t.getState(TYPE_VOTER, voterId, &v)
	return v
}


func (t *StateDAO) DeleteDecision(ballotId, decisionId string){
	t.deleteState(TYPE_RESULTS, ballotId+"/"+decisionId);
	t.deleteState(TYPE_DECISION, ballotId+"/"+decisionId);
}

func (t *StateDAO) DeleteBallot(ballotId string){
	t.deleteState(TYPE_BALLOT, ballotId);
	t.removeBallotFromAccountBallots(ballotId)
}

func (t *StateDAO) GetBallot(ballotId string)(Ballot){
	var b Ballot
	t.getState(TYPE_BALLOT, ballotId, &b)
	return b
}

func (t *StateDAO) GetBallotDecisions(ballotId string)(BallotDecisions){
	ballot := t.GetBallot(ballotId)

	bDecisions := make([]Decision,0)
	for _, decisionId := range ballot.Decisions{
		d := t.GetDecision(ballotId, decisionId)
		bDecisions = append(bDecisions, d)
	}

	return BallotDecisions { Ballot: ballot, Decisions: bDecisions }
}

func (t *StateDAO) GetAccountBallots()(AccountBallots){
	var accountBallots AccountBallots
	t.getState(TYPE_ACCOUNT_BALLOTS, t.getAccountId(), &accountBallots)
	return accountBallots
}

func (t *StateDAO) saveState(objectType string, id string, object interface{}){
	var json_bytes, err = json.Marshal(object)
	if err != nil {
		doPanic(500, "Invalid JSON while saving results")
	}
	put_err := t.Stub.PutState(t.getKey(objectType, id), json_bytes)
	if(put_err != nil){
		doPanic(500, "Error while putting type:"+objectType+", id:"+id)
	}
}

func (t *StateDAO) removeBallotFromAccountBallots(ballotId string){
	accountBallots := t.GetAccountBallots()
	delete(accountBallots.PublicBallotIds, ballotId)
	t.saveState(TYPE_ACCOUNT_BALLOTS, accountBallots.Id, accountBallots)
}

func (t *StateDAO) addToAccountBallots(ballot Ballot){
	accountBallots := t.GetAccountBallots()
	account_id := t.getAccountId()
	if(accountBallots.Id != account_id){
		accountBallots = AccountBallots{Id: account_id, PublicBallotIds: make(map[string]bool), PrivateBallotIds: make(map[string]bool)}
	}
	if(ballot.Private == true){
		accountBallots.PrivateBallotIds[ballot.Id] = true
		delete(accountBallots.PublicBallotIds, ballot.Id)
	}else {
		accountBallots.PublicBallotIds[ballot.Id] = true
		delete(accountBallots.PrivateBallotIds, ballot.Id)
	}
	t.saveState(TYPE_ACCOUNT_BALLOTS, account_id, accountBallots)
}


func (t *StateDAO) SaveDecisionResults(ballotId string, decision DecisionResults){
	t.saveState(TYPE_RESULTS, ballotId+"/"+decision.Id, decision)
}

func (t *StateDAO) SaveBallot(ballot Ballot){
	t.saveState(TYPE_BALLOT, ballot.Id, ballot)
	t.addToAccountBallots(ballot)
}

func (t *StateDAO) SaveVoter(v Voter){
	t.saveState(TYPE_VOTER, v.Id, v)
}

func (t *StateDAO) SaveDecision(decision Decision){
	t.saveState(TYPE_DECISION, decision.BallotId+"/"+decision.Id, decision)
}