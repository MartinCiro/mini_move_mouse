package controller

// ServiceAccountCredentials contiene las credenciales de la Service Account embebidas
// INSTRUCCIONES: Copia el contenido completo de tu service-account.json y reemplaza el string below
const ServiceAccountCredentials = `{
  "type": "service_account",
  "project_id": "tu-project-id",
  "private_key_id": "TU_PRIVATE_KEY_ID_AQUI",
  "private_key": "-----BEGIN PRIVATE KEY-----\nTU_PRIVATE_KEY_AQUI\n-----END PRIVATE KEY-----\n",
  "client_email": "tu-service-account@tu-project-id.iam.gserviceaccount.com",
  "client_id": "TU_CLIENT_ID_AQUI",
  "auth_uri": "https://accounts.google.com/o/oauth2/auth",
  "token_uri": "https://oauth2.googleapis.com/token",
  "auth_provider_x509_cert_url": "https://www.googleapis.com/oauth2/v1/certs",
  "client_x509_cert_url": "https://www.googleapis.com/robot/v1/metadata/x509/tu-service-account%40tu-project-id.iam.gserviceaccount.com"
}`
