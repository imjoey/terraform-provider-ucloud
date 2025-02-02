package ucloud

import (
	"fmt"
	"strings"
	"time"

	"github.com/hashicorp/terraform/helper/resource"
	"github.com/hashicorp/terraform/helper/schema"
	"github.com/hashicorp/terraform/helper/validation"

	"github.com/ucloud/ucloud-sdk-go/ucloud"
)

func resourceUCloudEIP() *schema.Resource {
	return &schema.Resource{
		Create: resourceUCloudEIPCreate,
		Read:   resourceUCloudEIPRead,
		Update: resourceUCloudEIPUpdate,
		Delete: resourceUCloudEIPDelete,
		Importer: &schema.ResourceImporter{
			State: schema.ImportStatePassthrough,
		},

		Schema: map[string]*schema.Schema{
			"bandwidth": {
				Type:         schema.TypeInt,
				Optional:     true,
				Computed:     true,
				ValidateFunc: validation.IntBetween(1, 800),
			},

			"internet_type": {
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
				ValidateFunc: validation.StringInSlice([]string{
					"bgp",
					"international",
				}, false),
			},

			"charge_type": {
				Type:     schema.TypeString,
				Optional: true,
				Default:  "month",
				ForceNew: true,
				ValidateFunc: validation.StringInSlice([]string{
					"month",
					"year",
					"dynamic",
				}, false),
			},

			"charge_mode": {
				Type:     schema.TypeString,
				Optional: true,
				Computed: true,
				ValidateFunc: validation.StringInSlice([]string{
					"traffic",
					"bandwidth",
				}, false),
			},

			"duration": {
				Type:         schema.TypeInt,
				Optional:     true,
				ForceNew:     true,
				ValidateFunc: validateDuration,
			},

			"name": {
				Type:         schema.TypeString,
				Optional:     true,
				Computed:     true,
				ValidateFunc: validateName,
			},

			"remark": {
				Type:     schema.TypeString,
				Optional: true,
				Computed: true,
			},

			"tag": {
				Type:         schema.TypeString,
				Optional:     true,
				Default:      defaultTag,
				ValidateFunc: validateTag,
				StateFunc:    stateFuncTag,
			},

			"status": {
				Type:     schema.TypeString,
				Computed: true,
			},

			"public_ip": {
				Type:     schema.TypeString,
				Computed: true,
			},

			"ip_set": {
				Type:     schema.TypeList,
				Computed: true,
				MaxItems: 2,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"ip": {
							Type:     schema.TypeString,
							Computed: true,
						},

						"internet_type": {
							Type:     schema.TypeString,
							Computed: true,
						},
					},
				},
			},

			"resource": {
				Type:     schema.TypeMap,
				Computed: true,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"type": {
							Type:     schema.TypeString,
							Computed: true,
						},

						"id": {
							Type:     schema.TypeString,
							Computed: true,
						},
					},
				},
			},

			"create_time": {
				Type:     schema.TypeString,
				Computed: true,
			},

			"expire_time": {
				Type:     schema.TypeString,
				Computed: true,
			},
		},
	}
}

func resourceUCloudEIPCreate(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*UCloudClient)
	conn := client.unetconn

	req := conn.NewAllocateEIPRequest()
	req.ChargeType = ucloud.String(upperCamelCvt.unconvert(d.Get("charge_type").(string)))
	req.PayMode = ucloud.String(upperCamelCvt.unconvert(d.Get("charge_mode").(string)))
	req.OperatorName = ucloud.String(upperCamelCvt.unconvert(d.Get("internet_type").(string)))

	if v, ok := d.GetOk("charge_mode"); ok {
		req.PayMode = ucloud.String(upperCamelCvt.unconvert(v.(string)))
	} else {
		req.PayMode = ucloud.String(upperCamelCvt.unconvert("bandwidth"))
	}

	if v, ok := d.GetOk("bandwidth"); ok {
		req.Bandwidth = ucloud.Int(v.(int))
	} else {
		req.Bandwidth = ucloud.Int(1)
	}

	if v, ok := d.GetOk("duration"); ok {
		req.Quantity = ucloud.Int(v.(int))
	} else {
		req.Quantity = ucloud.Int(1)
	}

	if v, ok := d.GetOk("name"); ok {
		req.Name = ucloud.String(v.(string))
	} else {
		req.Name = ucloud.String(resource.PrefixedUniqueId("tf-eip-"))
	}

	// if tag is empty string, use default tag
	if v, ok := d.GetOk("tag"); ok {
		req.Tag = ucloud.String(v.(string))
	} else {
		req.Tag = ucloud.String(defaultTag)
	}

	if v, ok := d.GetOk("remark"); ok {
		req.Remark = ucloud.String(v.(string))
	}

	resp, err := conn.AllocateEIP(req)
	if err != nil {
		return fmt.Errorf("error on creating eip, %s", err)
	}

	if len(resp.EIPSet) != 1 {
		return fmt.Errorf("error on creating eip, expected exactly one eip, got %v", len(resp.EIPSet))
	}

	d.SetId(resp.EIPSet[0].EIPId)

	// after create eip, we need to wait it initialized
	stateConf := eipWaitForState(client, d.Id())

	_, err = stateConf.WaitForState()
	if err != nil {
		return fmt.Errorf("error on waiting for eip %q complete creating, %s", d.Id(), err)
	}

	return resourceUCloudEIPRead(d, meta)
}

func resourceUCloudEIPUpdate(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*UCloudClient)
	conn := client.unetconn

	d.Partial(true)

	if d.HasChange("bandwidth") && !d.IsNewResource() {
		reqBand := conn.NewModifyEIPBandwidthRequest()
		reqBand.EIPId = ucloud.String(d.Id())
		reqBand.Bandwidth = ucloud.Int(d.Get("bandwidth").(int))

		_, err := conn.ModifyEIPBandwidth(reqBand)
		if err != nil {
			return fmt.Errorf("error on %s to eip %q, %s", "ModifyEIPBandwidth", d.Id(), err)
		}

		d.SetPartial("bandwidth")

		// after update eip bandwidth, we need to wait it completed
		stateConf := eipWaitForState(client, d.Id())

		_, err = stateConf.WaitForState()
		if err != nil {
			return fmt.Errorf("error on waiting for %s complete to eip %q, %s", "ModifyEIPBandwidth", d.Id(), err)
		}
	}

	if d.HasChange("charge_mode") && !d.IsNewResource() {
		reqCharge := conn.NewSetEIPPayModeRequest()
		reqCharge.EIPId = ucloud.String(d.Id())
		reqCharge.PayMode = ucloud.String(upperCamelCvt.unconvert(d.Get("charge_mode").(string)))
		reqCharge.Bandwidth = ucloud.Int(d.Get("bandwidth").(int))

		_, err := conn.SetEIPPayMode(reqCharge)
		if err != nil {
			return fmt.Errorf("error on %s to eip %q, %s", "SetEIPPayMode", d.Id(), err)
		}

		d.SetPartial("charge_mode")

		// after update eip internet charge mode, we need to wait it completed
		stateConf := eipWaitForState(client, d.Id())

		_, err = stateConf.WaitForState()
		if err != nil {
			return fmt.Errorf("error on waiting for %s complete to eip %q, %s", "SetEIPPayMode", d.Id(), err)
		}
	}

	isChanged := false
	reqAttribute := conn.NewUpdateEIPAttributeRequest()
	reqAttribute.EIPId = ucloud.String(d.Id())

	if d.HasChange("name") && !d.IsNewResource() {
		reqAttribute.Name = ucloud.String(d.Get("name").(string))
		isChanged = true
	}

	if d.HasChange("tag") && !d.IsNewResource() {
		isChanged = true

		// if tag is empty string, use default tag
		if v, ok := d.GetOk("tag"); ok {
			reqAttribute.Tag = ucloud.String(v.(string))
		} else {
			reqAttribute.Tag = ucloud.String(defaultTag)
		}
	}

	if d.HasChange("remark") && !d.IsNewResource() {
		reqAttribute.Remark = ucloud.String(d.Get("remark").(string))
		isChanged = true
	}

	if isChanged {
		_, err := conn.UpdateEIPAttribute(reqAttribute)
		if err != nil {
			return fmt.Errorf("error on %s to eip %q, %s", "UpdateEIPAttribute", d.Id(), err)
		}

		d.SetPartial("name")
		d.SetPartial("tag")
		d.SetPartial("remark")

		// after eip update eip attribute, we need to wait it completed
		stateConf := eipWaitForState(client, d.Id())

		_, err = stateConf.WaitForState()
		if err != nil {
			return fmt.Errorf("error on waiting for %s complete to eip %q, %s", "UpdateEIPAttribute", d.Id(), err)
		}
	}

	d.Partial(false)

	return resourceUCloudEIPRead(d, meta)
}

func resourceUCloudEIPRead(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*UCloudClient)

	eip, err := client.describeEIPById(d.Id())
	if err != nil {
		if isNotFoundError(err) {
			d.SetId("")
			return nil
		}
		return fmt.Errorf("error on reading eip %q, %s", d.Id(), err)
	}

	d.Set("bandwidth", eip.Bandwidth)
	d.Set("charge_type", upperCamelCvt.convert(eip.ChargeType))
	d.Set("charge_mode", upperCamelCvt.convert(eip.PayMode))
	d.Set("name", eip.Name)
	d.Set("remark", eip.Remark)
	d.Set("tag", eip.Tag)
	d.Set("status", eip.Status)
	d.Set("create_time", timestampToString(eip.CreateTime))
	d.Set("expire_time", timestampToString(eip.ExpireTime))

	eipAddr := []map[string]interface{}{}
	for _, item := range eip.EIPAddr {
		eipAddr = append(eipAddr, map[string]interface{}{
			"ip":            item.IP,
			"internet_type": item.OperatorName,
		})

		d.Set("public_ip", item.IP)
		d.Set("internet_type", strings.ToLower(item.OperatorName))
	}

	if err := d.Set("ip_set", eipAddr); err != nil {
		return err
	}

	if err := d.Set("resource", map[string]string{
		"type": lowerCaseProdCvt.unconvert(eip.Resource.ResourceType),
		"id":   eip.Resource.ResourceId,
	}); err != nil {
		return err
	}

	return nil
}

func resourceUCloudEIPDelete(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*UCloudClient)
	conn := client.unetconn

	req := conn.NewReleaseEIPRequest()
	req.EIPId = ucloud.String(d.Id())

	return resource.Retry(5*time.Minute, func() *resource.RetryError {
		if _, err := conn.ReleaseEIP(req); err != nil {
			return resource.NonRetryableError(fmt.Errorf("error on deleting eip %q, %s", d.Id(), err))
		}

		_, err := client.describeEIPById(d.Id())
		if err != nil {
			if isNotFoundError(err) {
				return nil
			}
			return resource.NonRetryableError(fmt.Errorf("error on reading eip when deleting %q, %s", d.Id(), err))
		}

		return resource.RetryableError(fmt.Errorf("the specified eip %q has not been deleted due to unknown error", d.Id()))
	})
}

func eipWaitForState(client *UCloudClient, eipId string) *resource.StateChangeConf {
	return &resource.StateChangeConf{
		Pending:    []string{statusPending},
		Target:     []string{"free", "used"},
		Timeout:    2 * time.Minute,
		Delay:      2 * time.Second,
		MinTimeout: 1 * time.Second,
		Refresh: func() (interface{}, string, error) {
			eip, err := client.describeEIPById(eipId)
			if err != nil {
				if isNotFoundError(err) {
					return nil, statusPending, nil
				}
				return nil, "", err
			}

			state := eip.Status
			if !isStringIn(state, []string{"free", "used"}) {
				state = statusPending
			}

			return eip, state, nil
		},
	}
}
