const ws = new WebSocket('ws://' + window.location.host + '/ws');

const submit = document.getElementById('submit');

ws.onmessage = event => {
    displayMessages(event)
};


ws.onopen = async event => {

    const data =  await fetch('./history')
        .then(function (response) {
            console.log('DATA', response);
            return response.text();

        }).then(function (payload) {
            const data = payload.trim().split('\n').map(JSON.parse)
            return data
 
        });
    // console.log('DATA', result);
    const history = document.createElement("div")
    const currentDiv = document.getElementById("input_div");
    data.map( e => {
        const p = document.createElement('p')   
        const message = document.createTextNode(`${e.username} : ${ e.message} `)
        document.body.insertBefore(history, currentDiv)
        history.appendChild(p);
        history.appendChild(message);
    })

}
const sendMessage = event => {
    const usersname = document.getElementById('username').value;
    if (!usersname) {
        alert('nuh, tell us your username first')
        return
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
    // chat.setAttribute('style', 'height: 500px')
    document.body.insertBefore(chat, currentDiv)
    chat.appendChild(username);
    chat.appendChild(message);
}



submit.addEventListener('submit', sendMessage);