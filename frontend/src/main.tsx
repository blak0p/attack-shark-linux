import { createRoot } from "react-dom/client";
import "./i18n";
import { App } from "./App";
import "./styles.css";
import { desktopService } from "./wails-service";

createRoot(document.getElementById("root")!).render(<App service={desktopService} />);
