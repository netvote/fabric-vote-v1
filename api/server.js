let express = require('express');
let bodyParser = require("body-parser");
let netvote = require("./service/netvote");
let app = express();

const PREFIX = "/api/v1";

app.use(bodyParser.json());

app.post(PREFIX+'/ballot', function (req, res) {
    netvote.addBallot(req.body).then((result)=> {
        res.send(result)
    }).catch((err) =>{
        console.error(err)
        res.sendStatus(500)
    });
});

app.get(PREFIX+'/ballot/:ballotId', function (req, res) {
    netvote.getBallotConfig(req.params.ballotId).then((result)=> {
        res.send(result)
    }).catch((err) =>{
        res.sendStatus(500)
    });
});

app.listen(3000, (req, res) => {
    console.log('Example app listening on port 3000!')
});