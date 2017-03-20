let fabric = require("../fabric/fabric.js");
const uuidV4 = require('uuid/v4');
let admin = require("firebase-admin");

let firebase = admin.initializeApp({
    credential: admin.credential.cert({
        projectId: process.env.FIREBASE_PROJECT_ID,
        clientEmail: process.env.FIREBASE_CLIENT_EMAIL,
        privateKey: process.env.FIREBASE_PRIVATE_KEY
    }),
    databaseURL: process.env.FIREBASE_DATABASE_URL
});

module.exports.addBallot = (payload) => {
    let ballot = payload.ballot;
    return new Promise(function(resolve, reject) {

        ballot.Ballot.Id = uuidV4();

        for(let decision of ballot.Decisions){
            decision.Id = uuidV4();
        }

        fabric.invoke("add_ballot", ballot, (commitResult) => {
            firebase.database().ref(payload.callbackRef).update({status: commitResult.result})
        }).then((result) => {
            let ballotResults = JSON.parse(result.toString());
            ballotResults.Decisions = ballot.Decisions;
            console.log("result="+JSON.stringify(ballotResults));
            resolve(JSON.stringify(ballotResults))
        }).catch((err)=>{
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