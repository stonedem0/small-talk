const ws = new WebSocket('ws://' + window.location.host + '/ws');

const sendMessage = event => {
    const usersname = document.getElementById('username').value;
    const message = document.getElementById('message').value
    ws.send(JSON.stringify({
        username: usersname,

        message: message
    }))
    event.preventDefault();
}

ws.onmessage = function (event) {
    const chat = document.createElement("div");
    const username = document.createTextNode(`${document.getElementById('username').value}: `);
    const message = document.createTextNode(document.getElementById('message').value);
    chat.appendChild(username);
    chat.appendChild(message);
    const currentDiv = document.getElementById("input_div");
    document.body.insertBefore(chat, currentDiv.nextSibling)
};

const submit = document.getElementById('submit');
submit.addEventListener('submit', sendMessage);