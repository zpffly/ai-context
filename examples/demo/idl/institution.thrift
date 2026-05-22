namespace go demo.institution

struct VerifyInstitutionRequest {
  1: required string verify_request_id
  2: required string institution_no
}

struct VerifyInstitutionResponse {
  1: required bool accepted
}

struct SubmitInstitutionVerifyResultRequest {
  1: required string verify_request_id
  2: required string result
}

struct SubmitInstitutionVerifyResultResponse {
  1: required bool ok
}

struct UpdateMerchantRequest {
  1: required string merchant_id
}

struct UpdateMerchantResponse {
  1: required bool ok
}

/**
 * 机构号相关 RPC。
 */
service InstitutionService {
  // 发起机构号中台校验。
  VerifyInstitutionResponse VerifyInstitution(1: VerifyInstitutionRequest req)
}

service MerchantService {
  // 接收机构号中台异步回调结果。
  SubmitInstitutionVerifyResultResponse SubmitInstitutionVerifyResult(1: SubmitInstitutionVerifyResultRequest req)

  // 更新商户基础信息，不是机构号校验入口。
  UpdateMerchantResponse UpdateMerchant(1: UpdateMerchantRequest req)
}
