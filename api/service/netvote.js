let fabric = require("../fabric/fabric.js");

module.exports.addBallot = (ballot) => {
    return new Promise(function(resolve, reject) {
        fabric.invoke("add_ballot", ballot).then((result) => {
            resolve(result)
        }).catch((err)=>{
            reject(err)
        });
    });
};