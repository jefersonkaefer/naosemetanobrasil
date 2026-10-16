import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import { BrowserRouter } from "react-router";
import { AuthProvider } from "./application/auth";
import { HttpPlatform } from "./infrastructure/http-platform";
import { App } from "./App";
import "./styles.css";

const gateway = new HttpPlatform();
createRoot(document.getElementById("root")!).render(
  <StrictMode>
    <BrowserRouter>
      <AuthProvider gateway={gateway}>
        <App />
      </AuthProvider>
    </BrowserRouter>
  </StrictMode>,
);
