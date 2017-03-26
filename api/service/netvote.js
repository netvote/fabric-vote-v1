let fabric = require("../fabric/fabric.js");
const uuidV4 = require('uuid/v4');
let admin = require("firebase-admin");
let EventHub = require('fabric-client/lib/EventHub.js');

let eventhubUrl = process.env.EVENT_HUB_URL;

let firebase = admin.initializeApp({
    credential: admin.credential.cert({
        projectId: process.env.FIREBASE_PROJECT_ID,
        clientEmail: process.env.FIREBASE_CLIENT_EMAIL,
        privateKey: process.env.FIREBASE_PRIVATE_KEY
    }),
    databaseURL: process.env.FIREBASE_DATABASE_URL
});


let voteEventHub = new EventHub();
voteEventHub.setPeerAddr(eventhubUrl);
voteEventHub.connect();
voteEventHub.registerChaincodeEvent("netvote-dev", "VOTE", (voteEvent)=>{
    let eventJson = JSON.parse(voteEvent.encodeJSON());
    eventJson["payload"] = JSON.parse(voteEvent.getPayload().toString("utf8"));
    console.log("VOTE EVENT: "+JSON.stringify(eventJson));

    let ballotId = eventJson.payload.Ballot.Ballot.Attributes.ballotId;
    let currentResults = eventJson.payload.BallotResults.Results;
    let updates = {};

    for(let decisionId in currentResults){
        if(currentResults.hasOwnProperty(decisionId)){
            updates["/ballot-results/"+ballotId+"/decisions/"+decisionId+"/results"] = currentResults[decisionId].Results
        }
    }

    firebase.database().ref().update(updates).then(()=>{
        console.log("updated ballot results for "+ballotId)
    });

});


module.exports.castVote = (body) => {
    //TODO: validation
    let vote = body.payload;
    return new Promise(function(resolve, reject) {
        fabric.invoke("cast_votes", vote, (commitResult) => {
            firebase.database().ref(body.txRefPath).update({status: commitResult.result})
        }).then((result) => {
            resolve(result)
        }).catch((err) => {
            firebase.database().ref(body.txRefPath).update({status: "error"});
            reject(err)
        });
    });
};

module.exports.addBallot = (body) => {
    //TODO: validation
    let ballot = body.payload;
    return new Promise(function(resolve, reject) {
        ballot.Ballot.Id = uuidV4();

        for(let decision of ballot.Decisions){
            decision.Id = uuidV4();
        }

        fabric.invoke("add_ballot", ballot, (commitResult) => {
            firebase.database().ref(body.txRefPath).update({status: commitResult.result})
        }).then((result) => {
            let ballotResults = JSON.parse(result.toString());
            ballotResults.Decisions = ballot.Decisions;
            resolve(JSON.stringify(ballotResults))
        }).catch((err)=>{
            firebase.database().ref(body.txRefPath).update({status: "error"});
            reject(err)
        });
    });
};

module.exports.getBallotConfig = (ballotId) => {
    console.log("getting ballot: "+ballotId);
    return new Promise(function(resolve, reject) {
        fabric.invoke("get_admin_ballot", { Id: ballotId }).then((result) => {
            resolve(result)
        }).catch((err)=>{
            reject(err)
        });
    });
};