const ws = new WebSocket('ws://' + window.location.host + '/ws');

const submit = document.getElementById('submit');

ws.onmessage = event => {
    displayMessages(event)
};

let savedUsername = '';

//TODO: save username 

ws.onopen = async () => {

    // handling history
    const data = await fetch('./history')
        .then(function (response) {
            return response.text();

        }).then(function (payload) {
            const data = payload.trim().split('\n').map(JSON.parse)
            return data

        });
    const history = document.createElement("div")
    const currentDiv = document.getElementById("input_div");
    data.map(e => {
        const p = document.createElement('br')
        const message = document.createTextNode(`${e.username}: ${ e.message}`)
        history.appendChild(p);
        history.appendChild(message);
        document.body.insertBefore(history, currentDiv)
     
    })

}
const sendMessage = event => {
    const usersname = document.getElementById('username').value;
    if (!usersname) {
        alert('nuh, tell us your username first')
        // return
    }
    const message = document.getElementById('message').value
    ws.send(JSON.stringify({
        username: usersname,
        message: message
    }))
    event.preventDefault();
}

const displayMessages = event => {
    const data = JSON.parse(event.data)
    const chat = document.createElement("div")
    const currentDiv = document.getElementById("input_div");
    const username = document.createTextNode(`${data.username}: `);
    const message = document.createTextNode(data.message);
    document.body.insertBefore(chat, currentDiv)
    chat.appendChild(username);
    chat.appendChild(message);
}



submit.addEventListener('submit', sendMessage);