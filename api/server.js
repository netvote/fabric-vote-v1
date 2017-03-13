let express = require('express');
let bodyParser = require("body-parser");
let netvote = require("./service/netvote");
let app = express();

app.use(bodyParser.json());

app.post('/admin/ballot', function (req, res) {
    netvote.addBallot(req.body).then((result)=> {
        res.send(result)
    }).catch((err) =>{
        res.sendStatus(500)
    });
});

app.listen(3000, (req, res) => {
    console.log('Example app listening on port 3000!')
});