let fabric = require("../fabric/fabric.js");
const uuidV4 = require('uuid/v4');

module.exports.addBallot = (ballot) => {
    return new Promise(function(resolve, reject) {

        ballot.Ballot.Id = uuidV4();

        for(let decision of ballot.Decisions){
            decision.Id = uuidV4();
        }

        fabric.invoke("add_ballot", ballot).then((result) => {
            resolve(result)
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