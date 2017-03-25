let express = require('express');
let bodyParser = require("body-parser");
let netvote = require("./service/netvote");
let app = express();

const PREFIX = "/api/v1";

app.use(bodyParser.json());

app.post(PREFIX+'/castVote', function (req, res) {
    netvote.castVote(req.body).then((result)=> {
        res.send(result)
    }).catch((err) =>{
        console.error("Error while voting", err);
        res.status(err.Code).send(err);
    });
});

app.post(PREFIX+'/ballot', function (req, res) {
    netvote.addBallot(req.body).then((result)=> {
        res.send(result)
    }).catch((err) =>{
        console.error("Error while saving ballot", err);
        res.status(err.Code).send(err);
    });
});

app.get(PREFIX+'/ballot/:ballotId', function (req, res) {
    netvote.getBallotConfig(req.params.ballotId).then((result)=> {
        res.send(result)
    }).catch((err) =>{
        res.status(err.Code).send(err);
    });
});

app.listen(3000, (req, res) => {
    console.log('Netvote API listening on port 3000!')
});