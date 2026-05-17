import { BrowserRouter, Navigate, Route, Routes } from "react-router-dom";
import { AuthProvider } from "./auth/AuthContext";
import { AppLayout } from "./layout/AppLayout";
import { LoginPage } from "./pages/LoginPage";
import { ResourcesPage } from "./pages/ResourcesPage";
import { JobsPage } from "./pages/JobsPage";
import { CoschedPage } from "./pages/CoschedPage";
import { DashboardPage } from "./pages/DashboardPage";

export default function App() {
  return (
    <AuthProvider>
      <BrowserRouter>
        <Routes>
          <Route path="/login" element={<LoginPage />} />
          <Route element={<AppLayout />}>
            <Route index element={<Navigate to="/resources" replace />} />
            <Route path="resources" element={<ResourcesPage />} />
            <Route path="jobs" element={<JobsPage />} />
            <Route path="cosched" element={<CoschedPage />} />
            <Route path="dashboard" element={<DashboardPage />} />
          </Route>
          <Route path="*" element={<Navigate to="/resources" replace />} />
        </Routes>
      </BrowserRouter>
    </AuthProvider>
  );
}
