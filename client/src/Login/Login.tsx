import  { useState } from "react";
import "./Login.css";
import PrimaryButton from "../components/PrimaryButton";
import logo from "../assets/fella.png"; 
import { API_URL } from "../config";


const Popup = () => {
  const [username, setUsername] = useState<string>("");
  const [password, setPassword] = useState<string>("");

  const signIn = async () => {
    console.log(username, password);
    const response = await fetch(`${API_URL}/login`, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
      },
      body: JSON.stringify({ username, password }),
    });
    // console.log(response);
    const data = await response.json();
    console.log(data);
    // if (username.trim()) {
    //   localStorage.setItem("username", username.trim());
    //   setUsername(username.trim());
    // } else {
    //   alert("Please enter a valid username.");
    // }
  };


  return (
    <div id="login-overlay">
      <div id="login-container">
        <div className="login-body">
          <div className="icon">
            <img src={logo} alt="logo" />
          </div>
          <div className="input-group">
            <label htmlFor="username-input" className="form-label">
              Username:
            </label>
            <input
              id="username-input"
              type="text"
              className="form-input"
              value={username}
              onChange={(e) => setUsername(e.target.value)}
            />
            <label htmlFor="password-input" className="form-label">
              Password:
            </label>
            <input
              id="password-input"
              type="password" 
              className="form-input"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
            />
          </div>
          <PrimaryButton onClick={signIn}>Log In</PrimaryButton>
          <div className="register-link">
          <a href="/register">Don't have an account? Register</a>
        </div>
        </div>
   
      </div>
    </div>
  );
};

export default Popup;
