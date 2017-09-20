var request = require('browser-request')
request({method: 'POST', url: randomNode}, on_response) 
    function on_response(er, response, body) {
        console.log("JAJAJAJJ")
        console.log(response)
    }
