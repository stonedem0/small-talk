import  { useState } from "react";
import "./Login.css";
import PrimaryButton from "../components/PrimaryButton";
import logo from "../assets/fella.png"; 
import { API_URL } from "../config";


const Popup = () => {
  const [username, setUsername] = useState<string>("");
  const [password, setPassword] = useState<string>("");
  const [error, setError] = useState<string>("");
  const [showRegister, setShowRegister] = useState(false);
  const [registerSuccess, setRegisterSuccess] = useState(false);

  const signIn = async () => {
    setError("");
    const response = await fetch(`${API_URL}/login`, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
      },
      body: JSON.stringify({ username, password }),
    });

    const data = await response.json();
    if (data.error) {
      setError(data.error);
    } else {
      localStorage.setItem("username", username.trim());
      setUsername(username.trim());
    }
  };

  const register = async () => {
    setError("");
    setRegisterSuccess(false);
    const response = await fetch(`${API_URL}/register`, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
      },
      body: JSON.stringify({ username, password }),
    });
    const text = await response.text();
    try {
      const data = JSON.parse(text);
      if (data.error) {
        setError(data.error);
        return;
      }
    } catch {}
    if (response.ok) {
      setRegisterSuccess(true);
      setError("");
    } else {
      setError(text);
    }
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
          {showRegister ? (
            <>
              <PrimaryButton onClick={register}>Register</PrimaryButton>
              {registerSuccess && <p className="success-message">Registration successful! You can now log in.</p>}
              <div style={{ marginTop: 8 }}>
                <span className="toggle-link" style={{ color: '#3366cc', cursor: 'pointer', textDecoration: 'underline', background: 'none', border: 'none', padding: 0 }} onClick={() => { setShowRegister(false); setError(""); setRegisterSuccess(false); }}>
                  Back to Login
                </span>
              </div>
            </>
          ) : (
            <>
              <PrimaryButton onClick={signIn}>Log In</PrimaryButton>
              <div style={{ marginTop: 8 }}>
                <span className="toggle-link" style={{ color: '#3366cc', cursor: 'pointer', textDecoration: 'underline', background: 'none', border: 'none', padding: 0 }} onClick={() => { setShowRegister(true); setError(""); setRegisterSuccess(false); }}>
                  Don't have an account? Register
                </span>
              </div>
            </>
          )}
          {error && <p className="error-message">{error}</p>}
        </div>
      </div>
    </div>
  );
};

export default Popup;
