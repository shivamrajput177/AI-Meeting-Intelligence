import { Navigate, Route, Routes } from "react-router-dom";
import { AuthProvider } from "../hooks/useAuth";
import { ProtectedRoute } from "../components/ProtectedRoute";
import { Layout } from "./Layout";
import { LoginPage } from "../pages/LoginPage";
import { SignupPage } from "../pages/SignupPage";
import { MeetingsPage } from "../pages/MeetingsPage";
import { MeetingDetailPage } from "../pages/MeetingDetailPage";

export function App() {
  return (
    <AuthProvider>
      <Routes>
        <Route element={<Layout />}>
          <Route path="/login" element={<LoginPage />} />
          <Route path="/signup" element={<SignupPage />} />
          <Route element={<ProtectedRoute />}>
            <Route path="/meetings" element={<MeetingsPage />} />
            <Route path="/meetings/:id" element={<MeetingDetailPage />} />
          </Route>
          <Route path="*" element={<Navigate to="/meetings" replace />} />
        </Route>
      </Routes>
    </AuthProvider>
  );
}
