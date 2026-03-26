import { BrowserRouter, Routes, Route, Navigate } from 'react-router-dom';
import { AuthProvider, useAuth } from './hooks/useAuth';
import MainLayout from './components/Layout/MainLayout';
import Login from './pages/Login';
import Register from './pages/Register';
import ContainerList from './pages/ContainerList';
import ContainerDetail from './pages/ContainerDetail';
import ImageList from './pages/ImageList';
import NetworkVolume from './pages/NetworkVolume';
import './index.css';
import type { ReactElement } from 'react';

const PrivateRoute = ({ children }: { children: ReactElement }) => {
  const { isAuthenticated, isLoading } = useAuth();
  
  if (isLoading) {
    return (
      <div className="flex justify-center items-center min-h-screen bg-neutral-50">
        <div className="text-center">
          <div className="inline-block w-12 h-12 border-4 border-primary-500 border-t-transparent rounded-full animate-spin"></div>
          <p className="mt-4 text-neutral-600">加载中...</p>
        </div>
      </div>
    );
  }
  
  return isAuthenticated ? children : <Navigate to="/login" />;
};

const App = () => {
  return (
    <AuthProvider>
      <BrowserRouter>
        <Routes>
          <Route path='/login' element={<Login />} />
          <Route path='/register' element={<Register />} />
          <Route
            path='/'
            element={
              <PrivateRoute>
                <MainLayout>
                  <ContainerList />
                </MainLayout>
              </PrivateRoute>
            }
          />
          <Route
            path='/containers/:id'
            element={
              <PrivateRoute>
                <MainLayout>
                  <ContainerDetail />
                </MainLayout>
              </PrivateRoute>
            }
          />
          <Route
            path='/images'
            element={
              <PrivateRoute>
                <MainLayout>
                  <ImageList />
                </MainLayout>
              </PrivateRoute>
            }
          />
          <Route
            path='/infrastructure'
            element={
              <PrivateRoute>
                <MainLayout>
                  <NetworkVolume />
                </MainLayout>
              </PrivateRoute>
            }
          />
        </Routes>
      </BrowserRouter>
    </AuthProvider>
  );
};

export default App;
